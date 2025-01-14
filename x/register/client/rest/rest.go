package rest

import (
	"github.com/gorilla/mux"

	"github.com/cosmos/cosmos-sdk/client/context"
)

// REST Variable names
// nolint
const (
	RestNetworkAddr = "network"
	RestNumLimit    = "limit"
	RestMoniker     = "moniker"
	RestOwner       = "owner"
	RestQueryType   = "query_type"
)

// RegisterRoutes registers register-related REST handlers to a router
func RegisterRoutes(cliCtx context.CLIContext, r *mux.Router) {
	registerTxRoutes(cliCtx, r)
	registerQueryRoutes(cliCtx, r)
}
