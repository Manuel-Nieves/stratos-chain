package simulation

// DONTCOVER

import (
	"fmt"
	"math/rand"

	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/cosmos/cosmos-sdk/x/simulation"

	"github.com/stratosnet/stratos-chain/x/evm/types"
)

const (
	keyExtraEIPs = "ExtraEIPs"
)

// ParamChanges defines the parameters that can be modified by param change proposals
// on the simulation.
func ParamChanges(r *rand.Rand) []simtypes.ParamChange {
	return []simtypes.ParamChange{
		simulation.NewSimParamChange(types.ModuleName, keyExtraEIPs,
			func(r *rand.Rand) string {
				return fmt.Sprintf("\"%d\"", genExtraEIPs(r))
			},
		),
	}
}