package keeper

import (
	"github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stratosnet/stratos-chain/x/pot/types"
)

// QueryPotRewardsParams Params for query 'custom/pot/rewards'
type QueryPotRewardsParams struct {
	Page     int
	Limit    int
	NodeAddr sdk.AccAddress
	Epoch    sdk.Int
}

// NewQueryPotRewardsParams creates a new instance of QueryPotRewardsParams
func NewQueryPotRewardsParams(page, limit int, nodeAddr sdk.AccAddress, epoch sdk.Int) QueryPotRewardsParams {
	return QueryPotRewardsParams{
		Page:     page,
		Limit:    limit,
		NodeAddr: nodeAddr,
		Epoch:    epoch,
	}
}

// QueryPotRewardsByepochParams Params for query 'custom/pot/rewards'
type QueryPotRewardsByepochParams struct {
	Page      int
	Limit     int
	OwnerAddr sdk.AccAddress
	Epoch     sdk.Int
}

// NewQueryPotRewardsByepochParams creates a new instance of QueryPotRewardsParams
func NewQueryPotRewardsByepochParams(page, limit int, ownerAddr sdk.AccAddress, epoch sdk.Int) QueryPotRewardsByepochParams {
	return QueryPotRewardsByepochParams{
		Page:      page,
		Limit:     limit,
		OwnerAddr: ownerAddr,
		Epoch:     epoch,
	}
}

// QueryPotRewardsByOwnerParams Params for query 'custom/pot/rewards/owner/<NodeWalletAddress>'
type QueryPotRewardsByOwnerParams struct {
	Page      int
	Limit     int
	OwnerAddr sdk.AccAddress
	Height    int64
}

// NewQueryPotRewardsByOwnerParams creates a new instance of QueryPotRewardsParams
func NewQueryPotRewardsByOwnerParams(page, limit int, ownerAddr sdk.AccAddress, height int64) QueryPotRewardsByOwnerParams {
	return QueryPotRewardsByOwnerParams{
		Page:      page,
		Limit:     limit,
		OwnerAddr: ownerAddr,
		Height:    height,
	}
}

type NodeRewardsInfo struct {
	NodeAddress         sdk.AccAddress
	Epoch               sdk.Int
	IndividualRewards   sdk.Coin
	MatureTotalReward   sdk.Coin
	ImmatureTotalReward sdk.Coin
}

// NewNodeRewardsInfo creates a new instance of NodeRewardsInfo
func NewNodeRewardsInfo(
	nodeAddress sdk.AccAddress,
	epoch sdk.Int,
	individualRewards,
	matureTotal,
	immatureTotal sdk.Int,
) NodeRewardsInfo {
	denomName := "ustos"
	return NodeRewardsInfo{
		NodeAddress:         nodeAddress,
		Epoch:               epoch,
		IndividualRewards:   sdk.NewCoin(denomName, individualRewards),
		MatureTotalReward:   sdk.NewCoin(denomName, matureTotal),
		ImmatureTotalReward: sdk.NewCoin(denomName, immatureTotal),
	}
}

func (k Keeper) GetNodesRewards(ctx sdk.Context, params QueryPotRewardsParams) (res []NodeRewardsInfo) {

	rewardAddrList := k.GetRewardAddressPool(ctx)

	for _, n := range rewardAddrList {
		if !(n.Equals(params.NodeAddr)) {
			continue
		}

		individualRewards := k.GetIndividualReward(ctx, n, params.Epoch)
		matureTotal := k.GetMatureTotalReward(ctx, n)
		immatureTotal := k.GetImmatureTotalReward(ctx, n)
		individualResult := NewNodeRewardsInfo(
			n,
			params.Epoch,
			individualRewards,
			matureTotal,
			immatureTotal,
		)

		res = append(res, individualResult)
	}

	start, end := client.Paginate(len(res), params.Page, params.Limit, QueryDefaultLimit)
	if start < 0 || end < 0 {
		return []NodeRewardsInfo{}
	} else {
		res = res[start:end]
		return res
	}
}

func (k Keeper) GetPotRewardsByEpoch(ctx sdk.Context, params QueryPotRewardsByepochParams) (res []types.Reward) {
	filteredNodesAddrStr := getFilteredNodesAddrStringByOwner(ctx, params.OwnerAddr, k)

	epochRewards := k.GetEpochReward(ctx, params.Epoch)
	for _, v := range epochRewards {
		if stringInSlice(v.NodeAddress.String(), filteredNodesAddrStr) {
			newNodeReward := types.NewReward(v.NodeAddress, v.RewardFromMiningPool, v.RewardFromTrafficPool)
			res = append(res, newNodeReward)
		}

	}
	start, end := client.Paginate(len(res), params.Page, params.Limit, QueryDefaultLimit)
	if start < 0 || end < 0 {
		return nil
	} else {
		res = res[start:end]
		return res
	}
}

func (k Keeper) GetNodesRewardsByOwner(ctx sdk.Context, params QueryPotRewardsByOwnerParams) (res []NodeRewardsInfo) {
	rewardAddrList := k.GetRewardAddressPool(ctx)
	filteredNodesAddrStr := getFilteredNodesAddrStringByOwner(ctx, params.OwnerAddr, k)
	epoch := k.getLastMaturedEpoch(ctx)

	for _, n := range rewardAddrList {
		if stringInSlice(n.String(), filteredNodesAddrStr) {
			individualRewards := k.GetIndividualReward(ctx, n, epoch)
			matureTotal := k.GetMatureTotalReward(ctx, n)
			immatureTotal := k.GetImmatureTotalReward(ctx, n)

			individualResult := NewNodeRewardsInfo(
				n,
				epoch,
				individualRewards,
				matureTotal,
				immatureTotal,
			)
			res = append(res, individualResult)
		}
	}
	start, end := client.Paginate(len(res), params.Page, params.Limit, QueryDefaultLimit)
	if start < 0 || end < 0 {
		return []NodeRewardsInfo{}
	} else {
		res = res[start:end]
		return res
	}
}

func getFilteredNodesAddrStringByOwner(ctx sdk.Context, ownerAddress sdk.AccAddress, k Keeper) []string {
	resourceNodesAddr := k.RegisterKeeper.GetAllResourceNodes(ctx)
	indexingNodesAddr := k.RegisterKeeper.GetAllIndexingNodes(ctx)
	filteredNodesAddrStr := make([]string, 0, len(resourceNodesAddr)+len(indexingNodesAddr))

	for _, n := range resourceNodesAddr {
		// match OwnerAddr (if supplied)
		if ownerAddress.Empty() || n.OwnerAddress.Equals(ownerAddress) {
			filteredNodesAddrStr = append(filteredNodesAddrStr, sdk.AccAddress(n.PubKey.Address()).String())
		}

	}
	for _, n := range indexingNodesAddr {
		// match OwnerAddr (if supplied)
		if ownerAddress.Empty() || n.OwnerAddress.Equals(ownerAddress) {
			filteredNodesAddrStr = append(filteredNodesAddrStr, sdk.AccAddress(n.PubKey.Address()).String())
		}
	}
	return filteredNodesAddrStr
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}
