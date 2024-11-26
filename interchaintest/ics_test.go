package e2e

import (
	"context"
	"testing"

	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/relayer"
	"github.com/strangelove-ventures/interchaintest/v8/testreporter"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

var (
	icsPath = "ics-path"
	// cd testing/consumer && DOCKER_BUILDKIT=0 docker build . --tag consumer:local
	ConsumerTestingChain = interchaintest.ChainSpec{
		Name:          "consumer",
		Version:       "local",
		ChainName:     "consumer",
		NumValidators: &vals, NumFullNodes: &fNodes,
		ChainConfig: ibc.ChainConfig{
			GasAdjustment:  3.0,
			TrustingPeriod: "504h",
			Type:           "cosmos",
			Name:           "consumer",
			ChainID:        "localchain-2",
			Bin:            "consumerd",
			Denom:          "utoken",
			CoinType:       "118",
			Bech32Prefix:   "cosmos",
			GasPrices:      "0.0" + "utoken",
			InterchainSecurityConfig: ibc.ICSConfig{
				ProviderVerOverride: "v0.0.0",
				ConsumerVerOverride: "v0.0.0",
			},
			Images: []ibc.DockerImage{
				ibc.NewDockerImage("consumer", "local", "1026:1026"),
			},
			ModifyGenesis: cosmos.ModifyGenesis([]cosmos.GenesisKV{
				cosmos.NewGenesisKV("app_state.gov.params.voting_period", VotingPeriod),
				cosmos.NewGenesisKV("app_state.gov.params.max_deposit_period", MaxDepositPeriod),
				cosmos.NewGenesisKV("app_state.gov.params.min_deposit.0.denom", Denom),
				cosmos.NewGenesisKV("app_state.gov.params.min_deposit.0.amount", "1"),
			}),
		},
	}
)

func TestICSConnection(t *testing.T) {
	ctx := context.Background()
	rep := testreporter.NewNopReporter()
	eRep := rep.RelayerExecReporter(t)
	client, network := interchaintest.DockerSetup(t)

	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		&DefaultChainSpec,
		&ConsumerTestingChain,
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	provider := chains[0].(*cosmos.CosmosChain)
	consumer := chains[1].(*cosmos.CosmosChain)

	// Relayer Factory
	r := interchaintest.NewBuiltinRelayerFactory(
		ibc.CosmosRly,
		zaptest.NewLogger(t),
		relayer.CustomDockerImage(RelayerRepo, RelayerVersion, "100:1000"),
		relayer.StartupFlags("--block-history", "200"),
	).Build(t, client, network)

	// Setup Interchain
	ic := interchaintest.NewInterchain().
		AddChain(provider).
		AddChain(consumer).
		AddRelayer(r, "rly").
		AddProviderConsumerLink(interchaintest.ProviderConsumerLink{
			Provider: provider,
			Consumer: consumer,
			Relayer:  r,
			Path:     icsPath,
		})

		// failed to start chains: failed to start provider chain prysm: failed to submit consumer addition proposal: exit
		// use Violets latest patch>?
	require.NoError(t, ic.Build(ctx, eRep, interchaintest.InterchainBuildOptions{
		TestName:         t.Name(),
		Client:           client,
		NetworkID:        network,
		SkipPathCreation: false,
	}))
	t.Cleanup(func() {
		_ = ic.Close()
	})

	// require.NoError(t, provider.FinishICSProviderSetup(ctx, r, eRep, icsPath))

	// amt := math.NewInt(10_000_000)
	// users := interchaintest.GetAndFundTestUsers(t, ctx, "default", amt,
	// provider,
	// consumer,
	// )
	//
	// userProvider := users[0]
	// userConsumer := users[1]

	// t.Run("validate funding", func(t *testing.T) {
	// 	bal, err := consumer.BankQueryBalance(ctx, userConsumer.FormattedAddress(), consumer.Config().Denom)
	// 	require.NoError(t, err)
	// 	require.EqualValues(t, amt, bal)

	// 	bal, err = provider.BankQueryBalance(ctx, userProvider.FormattedAddress(), provider.Config().Denom)
	// 	require.NoError(t, err)
	// 	require.EqualValues(t, amt, bal)
	// })

	// t.Run("provider -> consumer IBC transfer", func(t *testing.T) {
	// 	providerChannelInfo, err := r.GetChannels(ctx, eRep, provider.Config().ChainID)
	// 	require.NoError(t, err)

	// 	channelID, err := getTransferChannel(providerChannelInfo)
	// 	require.NoError(t, err, providerChannelInfo)

	// 	consumerChannelInfo, err := r.GetChannels(ctx, eRep, chain.Config().ChainID)
	// 	require.NoError(t, err)

	// 	consumerChannelID, err := getTransferChannel(consumerChannelInfo)
	// 	require.NoError(t, err, consumerChannelInfo)

	// 	dstAddress := user.FormattedAddress()
	// 	sendAmt := math.NewInt(7)
	// 	transfer := ibc.WalletAmount{
	// 		Address: dstAddress,
	// 		Denom:   provider.Config().Denom,
	// 		Amount:  sendAmt,
	// 	}

	// 	tx, err := provider.SendIBCTransfer(ctx, channelID, providerUser.KeyName(), transfer, ibc.TransferOptions{})
	// 	require.NoError(t, err)
	// 	require.NoError(t, tx.Validate())

	// 	require.NoError(t, r.Flush(ctx, eRep, icsPath, channelID))

	// 	srcDenomTrace := transfertypes.ParseDenomTrace(transfertypes.GetPrefixedDenom("transfer", consumerChannelID, provider.Config().Denom))
	// 	dstIbcDenom := srcDenomTrace.IBCDenom()

	// 	consumerBal, err := chain.BankQueryBalance(ctx, user.FormattedAddress(), dstIbcDenom)
	// 	require.NoError(t, err)
	// 	require.EqualValues(t, sendAmt, consumerBal)
	// })

}
