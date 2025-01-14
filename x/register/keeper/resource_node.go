package keeper

import (
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	stratos "github.com/stratosnet/stratos-chain/types"
	"github.com/stratosnet/stratos-chain/x/register/types"
	"github.com/tendermint/tendermint/crypto"
)

const resourceNodeCacheSize = 500

// Cache the amino decoding of resource nodes, as it can be the case that repeated slashing calls
// cause many calls to GetResourceNode, which were shown to throttle the state machine in our
// simulation. Note this is quite biased though, as the simulator does more slashes than a
// live chain should, however we require the slashing to be fast as no one pays gas for it.
type cachedResourceNode struct {
	resourceNode types.ResourceNode
	marshalled   string // marshalled amino bytes for the ResourceNode object (not address)
}

func newCachedResourceNode(resourceNode types.ResourceNode, marshalled string) cachedResourceNode {
	return cachedResourceNode{
		resourceNode: resourceNode,
		marshalled:   marshalled,
	}
}

// GetResourceNode get a single resource node
func (k Keeper) GetResourceNode(ctx sdk.Context, p2pAddress stratos.SdsAddress) (resourceNode types.ResourceNode, found bool) {
	store := ctx.KVStore(k.storeKey)
	value := store.Get(types.GetResourceNodeKey(p2pAddress))

	if value == nil {
		return resourceNode, false
	}

	// If these amino encoded bytes are in the cache, return the cached resource node
	strValue := string(value)
	if val, ok := k.resourceNodeCache[strValue]; ok {
		valToReturn := val.resourceNode
		return valToReturn, true
	}

	// amino bytes weren't found in cache, so amino unmarshal and add it to the cache
	resourceNode = types.MustUnmarshalResourceNode(k.cdc, value)
	cachedVal := newCachedResourceNode(resourceNode, strValue)
	k.resourceNodeCache[strValue] = newCachedResourceNode(resourceNode, strValue)
	k.resourceNodeCacheList.PushBack(cachedVal)

	// if the cache is too big, pop off the last element from it
	if k.resourceNodeCacheList.Len() > resourceNodeCacheSize {
		valToRemove := k.resourceNodeCacheList.Remove(k.resourceNodeCacheList.Front()).(cachedResourceNode)
		delete(k.resourceNodeCache, valToRemove.marshalled)
	}

	resourceNode = types.MustUnmarshalResourceNode(k.cdc, value)
	return resourceNode, true
}

// SetResourceNode sets the main record holding resource node details
func (k Keeper) SetResourceNode(ctx sdk.Context, resourceNode types.ResourceNode) {
	store := ctx.KVStore(k.storeKey)
	bz := types.MustMarshalResourceNode(k.cdc, resourceNode)
	store.Set(types.GetResourceNodeKey(resourceNode.GetNetworkAddr()), bz)
}

// GetAllResourceNodes get the set of all resource nodes with no limits, used during genesis dump
func (k Keeper) GetAllResourceNodes(ctx sdk.Context) (resourceNodes []types.ResourceNode) {
	store := ctx.KVStore(k.storeKey)
	iterator := sdk.KVStorePrefixIterator(store, types.ResourceNodeKey)
	defer iterator.Close()

	for ; iterator.Valid(); iterator.Next() {
		node := types.MustUnmarshalResourceNode(k.cdc, iterator.Value())
		resourceNodes = append(resourceNodes, node)
	}
	return resourceNodes
}

func (k Keeper) getResourceNodeIterator(ctx sdk.Context) sdk.Iterator {
	store := ctx.KVStore(k.storeKey)
	iterator := sdk.KVStorePrefixIterator(store, types.ResourceNodeKey)
	return iterator
}

// AddResourceNodeStake Update the tokens of an existing resource node
func (k Keeper) AddResourceNodeStake(ctx sdk.Context, resourceNode types.ResourceNode, tokenToAdd sdk.Coin,
) (ozoneLimitChange sdk.Int, err error) {

	coins := sdk.NewCoins(tokenToAdd)

	// sub coins from owner's wallet
	hasCoin := k.bankKeeper.HasCoins(ctx, resourceNode.GetOwnerAddr(), coins)
	if !hasCoin {
		return sdk.ZeroInt(), types.ErrInsufficientBalance
	}
	_, err = k.bankKeeper.SubtractCoins(ctx, resourceNode.GetOwnerAddr(), coins)
	if err != nil {
		return sdk.ZeroInt(), err
	}

	switch resourceNode.GetStatus() {
	case sdk.Unbonded:
		notBondedTokenInPool := k.GetResourceNodeNotBondedToken(ctx)
		notBondedTokenInPool = notBondedTokenInPool.Add(tokenToAdd)
		k.SetResourceNodeNotBondedToken(ctx, notBondedTokenInPool)
	case sdk.Bonded:
		bondedTokenInPool := k.GetResourceNodeBondedToken(ctx)
		bondedTokenInPool = bondedTokenInPool.Add(tokenToAdd)
		k.SetResourceNodeBondedToken(ctx, bondedTokenInPool)
	case sdk.Unbonding:
		return sdk.ZeroInt(), types.ErrUnbondingNode
	}

	resourceNode = resourceNode.AddToken(tokenToAdd.Amount)
	//resourceNode.Suspend = false

	// set status from unBonded to bonded & move stake from not bonded token pool to bonded token pool
	// since resource node registration does not require voting for now
	if resourceNode.Status.Equal(sdk.Unbonded) {
		resourceNode.Status = sdk.Bonded

		tokenToBond := sdk.NewCoin(k.BondDenom(ctx), resourceNode.GetTokens())
		notBondedToken := k.GetResourceNodeNotBondedToken(ctx)
		bondedToken := k.GetResourceNodeBondedToken(ctx)

		if notBondedToken.IsLT(tokenToBond) {
			return sdk.ZeroInt(), types.ErrInsufficientBalanceOfNotBondedPool
		}
		notBondedToken = notBondedToken.Sub(tokenToBond)
		bondedToken = bondedToken.Add(tokenToBond)
		k.SetResourceNodeNotBondedToken(ctx, notBondedToken)
		k.SetResourceNodeBondedToken(ctx, bondedToken)
	}

	k.SetResourceNode(ctx, resourceNode)
	ozoneLimitChange = k.increaseOzoneLimitByAddStake(ctx, tokenToAdd.Amount)

	return ozoneLimitChange, nil
}

func (k Keeper) RemoveTokenFromPoolWhileUnbondingResourceNode(ctx sdk.Context, resourceNode types.ResourceNode, tokenToSub sdk.Coin) error {
	// get pools
	bondedTokenInPool := k.GetResourceNodeBondedToken(ctx)
	notBondedTokenInPool := k.GetResourceNodeNotBondedToken(ctx)
	if bondedTokenInPool.IsLT(tokenToSub) {
		return types.ErrInsufficientBalanceOfBondedPool
	}
	// remove token from BondedPool
	bondedTokenInPool = bondedTokenInPool.Sub(tokenToSub)
	k.SetResourceNodeBondedToken(ctx, bondedTokenInPool)
	// add token into NotBondedPool
	notBondedTokenInPool = notBondedTokenInPool.Add(tokenToSub)
	k.SetResourceNodeNotBondedToken(ctx, notBondedTokenInPool)
	return nil
}

// SubtractResourceNodeStake Update the tokens of an existing resource node
func (k Keeper) SubtractResourceNodeStake(ctx sdk.Context, resourceNode types.ResourceNode, tokenToSub sdk.Coin) error {
	ownerAcc := k.accountKeeper.GetAccount(ctx, resourceNode.OwnerAddress)
	if ownerAcc == nil {
		return types.ErrNoOwnerAccountFound
	}

	coins := sdk.NewCoins(tokenToSub)

	if resourceNode.Tokens.LT(tokenToSub.Amount) {
		return types.ErrInsufficientBalance
	}

	// deduct tokens from NotBondedPool
	notBondedTokenInPool := k.GetResourceNodeNotBondedToken(ctx)
	if notBondedTokenInPool.IsLT(tokenToSub) {
		return types.ErrInsufficientBalanceOfNotBondedPool
	}
	notBondedTokenInPool = notBondedTokenInPool.Sub(tokenToSub)
	k.SetResourceNodeNotBondedToken(ctx, notBondedTokenInPool)

	// deduct slashing amount first
	coins = k.DeductSlashing(ctx, resourceNode.OwnerAddress, coins)
	// add tokens to owner acc
	_, err := k.bankKeeper.AddCoins(ctx, resourceNode.OwnerAddress, coins)
	if err != nil {
		return err
	}

	resourceNode = resourceNode.SubToken(tokenToSub.Amount)
	newStake := resourceNode.GetTokens()

	k.SetResourceNode(ctx, resourceNode)

	if newStake.IsZero() {
		err = k.removeResourceNode(ctx, resourceNode.GetNetworkAddr())
		if err != nil {
			return err
		}
	}
	return nil
}

// remove the resource node record and associated indexes
func (k Keeper) removeResourceNode(ctx sdk.Context, addr stratos.SdsAddress) error {
	// first retrieve the old resource node record
	resourceNode, found := k.GetResourceNode(ctx, addr)
	if !found {
		return types.ErrNoResourceNodeFound
	}

	if resourceNode.Tokens.IsPositive() {
		panic("attempting to remove a resource node which still contains tokens")
	}

	// delete the old resource node record
	store := ctx.KVStore(k.storeKey)
	store.Delete(types.GetResourceNodeKey(addr))
	return nil
}

func (k Keeper) RegisterResourceNode(ctx sdk.Context, networkAddr stratos.SdsAddress, pubKey crypto.PubKey, ownerAddr sdk.AccAddress,
	description types.Description, nodeType types.NodeType, stake sdk.Coin) (ozoneLimitChange sdk.Int, err error) {

	resourceNode := types.NewResourceNode(networkAddr, pubKey, ownerAddr, description, nodeType, ctx.BlockHeader().Time)
	ozoneLimitChange, err = k.AddResourceNodeStake(ctx, resourceNode, stake)
	return ozoneLimitChange, err
}

func (k Keeper) UpdateResourceNode(ctx sdk.Context, description types.Description, nodeType types.NodeType,
	networkAddr stratos.SdsAddress, ownerAddr sdk.AccAddress) error {

	node, found := k.GetResourceNode(ctx, networkAddr)
	if !found {
		return types.ErrNoResourceNodeFound
	}

	if !node.OwnerAddress.Equals(ownerAddr) {
		return types.ErrInvalidOwnerAddr
	}

	node.Description = description
	node.NodeType = nodeType

	k.SetResourceNode(ctx, node)

	return nil
}

func (k Keeper) UpdateResourceNodeStake(ctx sdk.Context, networkAddr stratos.SdsAddress, ownerAddr sdk.AccAddress,
	stakeDelta sdk.Coin, incrStake bool) (ozoneLimitChange sdk.Int, unbondingMatureTime time.Time, err error) {

	blockTime := ctx.BlockHeader().Time
	node, found := k.GetResourceNode(ctx, networkAddr)
	if !found {
		return sdk.ZeroInt(), blockTime, types.ErrNoResourceNodeFound
	}

	if !node.OwnerAddress.Equals(ownerAddr) {
		return sdk.ZeroInt(), blockTime, types.ErrInvalidOwnerAddr
	}

	if incrStake {
		ozoneLimitChange, err := k.AddResourceNodeStake(ctx, node, stakeDelta)
		if err != nil {
			return sdk.ZeroInt(), blockTime, err
		}
		return ozoneLimitChange, blockTime, nil
	} else {
		// if !incrStake
		if node.GetStatus() == sdk.Unbonding {
			return sdk.ZeroInt(), blockTime, types.ErrUnbondingNode
		}

		ozoneLimitChange, completionTime, err := k.UnbondResourceNode(ctx, node, stakeDelta.Amount)
		if err != nil {
			return sdk.ZeroInt(), blockTime, err
		}
		return ozoneLimitChange, completionTime, nil
	}
}

func (k Keeper) SetResourceNodeBondedToken(ctx sdk.Context, token sdk.Coin) {
	store := ctx.KVStore(k.storeKey)
	bz := k.cdc.MustMarshalBinaryLengthPrefixed(token)
	store.Set(types.ResourceNodeBondedTokenKey, bz)
}

func (k Keeper) GetResourceNodeBondedToken(ctx sdk.Context) (token sdk.Coin) {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.ResourceNodeBondedTokenKey)
	if bz == nil {
		return sdk.NewCoin(k.BondDenom(ctx), sdk.ZeroInt())
	}
	k.cdc.MustUnmarshalBinaryLengthPrefixed(bz, &token)
	return token
}

func (k Keeper) SetResourceNodeNotBondedToken(ctx sdk.Context, token sdk.Coin) {
	store := ctx.KVStore(k.storeKey)
	bz := k.cdc.MustMarshalBinaryLengthPrefixed(token)
	store.Set(types.ResourceNodeNotBondedTokenKey, bz)
}

func (k Keeper) GetResourceNodeNotBondedToken(ctx sdk.Context) (token sdk.Coin) {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.ResourceNodeNotBondedTokenKey)
	if bz == nil {
		return sdk.NewCoin(k.BondDenom(ctx), sdk.ZeroInt())
	}
	k.cdc.MustUnmarshalBinaryLengthPrefixed(bz, &token)
	return token
}
