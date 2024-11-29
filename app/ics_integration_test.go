package app_test

import (
	"encoding/json"
	"os"
	"testing"

	"cosmossdk.io/log"
	"cosmossdk.io/math"
	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	dbm "github.com/cosmos/cosmos-db"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	ibctesting "github.com/cosmos/ibc-go/v8/testing"
	appConsumer "github.com/cosmos/interchain-security/v6/app/consumer"
	"github.com/cosmos/interchain-security/v6/tests/integration"
	icstestingutils "github.com/cosmos/interchain-security/v6/testutil/ibc_testing"
	ccvtypes "github.com/cosmos/interchain-security/v6/x/ccv/types"
	prysmapp "github.com/lightlabs-dev/prysm/app"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

var ccvSuite *integration.CCVTestSuite

// Set in AppIniterTempDir
var app *prysmapp.ChainApp

func init() {
	// Pass in concrete app types that implement the interfaces defined in https://github.com/cosmos/interchain-security/testutil/integration/interfaces.go
	// IMPORTANT: the concrete app types passed in as type parameters here must match the
	// concrete app types returned by the relevant app initers.
	ccvSuite = integration.NewCCVTestSuite[*prysmapp.ChainApp, *appConsumer.App](AppIniterTempDir, icstestingutils.ConsumerAppIniter, []string{})
}

func TestCCVTestSuite(t *testing.T) {
	suite.Run(t, ccvSuite)
}

// Some tests require a random directory to be created when running IBC testing suite with the provider.
// This is due to how CosmWasmVM initializes the VM - all IBC testing apps must have different dirs so they don't conflict.
func AppIniterTempDir() (ibctesting.TestingApp, map[string]json.RawMessage) {
	tmpDir, err := os.MkdirTemp("", "")
	if err != nil {
		panic(err)
	}

	app = prysmapp.NewChainApp(
		log.NewNopLogger(),
		dbm.NewMemDB(),
		nil,
		true,
		simtestutil.NewAppOptionsWithFlagHome(tmpDir),
		[]wasmkeeper.Option{},
	)

	testApp := ibctesting.TestingApp(app)

	return testApp, app.DefaultGenesis()
}

func TestICSEpochs(t *testing.T) {
	// a bit hacky but cannot be called
	//  in SetupTest() since it requires `t`
	ccvSuite.SetT(t)
	ccvSuite.SetupTest()

	providerKeeper := app.GetProviderKeeper()
	stakingKeeper := app.StakingKeeper
	provCtx := ccvSuite.GetProviderChain().GetContext()

	delegateFn := func(ctx sdk.Context) {
		delAddr := ccvSuite.GetProviderChain().SenderAccount.GetAddress()
		consAddr := sdk.ConsAddress(ccvSuite.GetProviderChain().Vals.Validators[0].Address)
		validator, err := stakingKeeper.ValidatorByConsAddr(ctx, consAddr)
		require.NoError(t, err)
		_, err = stakingKeeper.Delegate(
			ctx,
			delAddr,
			math.NewInt(1000000),
			stakingtypes.Unbonded,
			validator.(stakingtypes.Validator),
			true,
		)
		require.NoError(t, err)
	}

	getVSCPacketsFn := func() []ccvtypes.ValidatorSetChangePacketData {
		consumerID := icstestingutils.FirstConsumerID
		return providerKeeper.GetPendingVSCPackets(provCtx, consumerID)
	}

	nextEpoch := func(ctx sdk.Context) sdk.Context {
		for {
			if providerKeeper.BlocksUntilNextEpoch(ctx) == 0 {
				return ctx
			}
			ccvSuite.GetProviderChain().NextBlock()
			ctx = ccvSuite.GetProviderChain().GetContext()
		}
	}

	// Bond some tokens on provider to change validator powers
	delegateFn(provCtx)

	// VSCPacket should only be created at the end of the current epoch
	require.Empty(t, getVSCPacketsFn())
	provCtx = nextEpoch(provCtx)
	// Expect to create a VSC packet
	// without sending it since CCV channel isn't established
	_, err := app.EndBlocker(provCtx)
	require.NoError(t, err)
	require.NotEmpty(t, getVSCPacketsFn())

	// Expect the VSC packet to send after setting up the CCV channel
	ccvSuite.SetupCCVChannel(ccvSuite.GetCCVPath())
	require.Empty(t, getVSCPacketsFn())
	// Expect VSC Packet to be committed
	require.Len(t, ccvSuite.GetProviderChain().App.GetIBCKeeper().ChannelKeeper.GetAllPacketCommitmentsAtChannel(
		provCtx,
		ccvSuite.GetCCVPath().EndpointB.ChannelConfig.PortID,
		ccvSuite.GetCCVPath().EndpointB.ChannelID,
	), 1)

	// Bond some tokens on provider to change validator powers
	delegateFn(provCtx)
	// Second VSCPacket should only be created at the end of the current epoch
	require.Empty(t, getVSCPacketsFn())

	provCtx = nextEpoch(provCtx)
	_, err = app.EndBlocker(provCtx)
	require.NoError(t, err)
	// Expect second VSC Packet to be committed
	require.Len(t, ccvSuite.GetProviderChain().App.GetIBCKeeper().ChannelKeeper.GetAllPacketCommitmentsAtChannel(
		provCtx,
		ccvSuite.GetCCVPath().EndpointB.ChannelConfig.PortID,
		ccvSuite.GetCCVPath().EndpointB.ChannelID,
	), 2)
}
