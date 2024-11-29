package app

import (
	capabilitykeeper "github.com/cosmos/ibc-go/modules/capability/keeper"
	ibckeeper "github.com/cosmos/ibc-go/v8/modules/core/keeper"

	"github.com/cosmos/cosmos-sdk/baseapp"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"

	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	ibcproviderkeeper "github.com/cosmos/interchain-security/v6/x/ccv/provider/keeper"

	"github.com/cosmos/cosmos-sdk/client"
	ibctestingtypes "github.com/cosmos/ibc-go/v8/testing/types"
	icstest "github.com/cosmos/interchain-security/v6/testutil/integration"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
)

func (app *ChainApp) GetProviderKeeper() ibcproviderkeeper.Keeper {
	return app.ProviderKeeper
}

func (app *ChainApp) GetIBCKeeper() *ibckeeper.Keeper {
	return app.IBCKeeper
}

func (app *ChainApp) GetScopedIBCKeeper() capabilitykeeper.ScopedKeeper {
	return app.ScopedIBCKeeper
}

func (app *ChainApp) GetBaseApp() *baseapp.BaseApp {
	return app.BaseApp
}

func (app *ChainApp) GetBankKeeper() bankkeeper.Keeper {
	return app.BankKeeper
}

func (app *ChainApp) GetAppStakingKeeper() *stakingkeeper.Keeper {
	return app.StakingKeeper
}

func (app *ChainApp) GetStakingKeeper() ibctestingtypes.StakingKeeper {
	return app.StakingKeeper
}

func (app *ChainApp) GetAccountKeeper() authkeeper.AccountKeeper {
	return app.AccountKeeper
}

func (app *ChainApp) GetWasmKeeper() wasmkeeper.Keeper {
	return app.WasmKeeper
}

// GetTxConfig implements the TestingApp interface.
func (app *ChainApp) GetTxConfig() client.TxConfig {
	return app.txConfig
}

// GetTestStakingKeeper implements the ProviderApp interface.
func (app *ChainApp) GetTestStakingKeeper() icstest.TestStakingKeeper { //nolint:nolintlint
	return app.StakingKeeper
}

// GetTestBankKeeper implements the ProviderApp interface.
func (app *ChainApp) GetTestBankKeeper() icstest.TestBankKeeper { //nolint:nolintlint
	return app.BankKeeper
}

// GetTestSlashingKeeper implements the ProviderApp interface.
func (app *ChainApp) GetTestSlashingKeeper() icstest.TestSlashingKeeper { //nolint:nolintlint
	return app.SlashingKeeper
}

// GetTestDistributionKeeper implements the ProviderApp interface.
func (app *ChainApp) GetTestDistributionKeeper() icstest.TestDistributionKeeper { //nolint:nolintlint
	return app.DistrKeeper
}

func (app *ChainApp) GetTestAccountKeeper() icstest.TestAccountKeeper { //nolint:nolintlint
	return app.AccountKeeper
}

// GetTestGovKeeper implements the TestingApp interface.
func (app *ChainApp) GetTestGovKeeper() *govkeeper.Keeper {
	return app.GovKeeper
}
