package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strconv"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	abcitypes "github.com/cometbft/cometbft/abci/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	clienttypes "github.com/cosmos/ibc-go/v8/modules/core/02-client/types"
	providertypes "github.com/cosmos/interchain-security/v5/x/ccv/provider/types"
	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/relayer"
	"github.com/strangelove-ventures/interchaintest/v8/testreporter"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"go.uber.org/zap/zaptest"
)

type ConsumerConfig struct {
	ChainName         string
	Version           string
	Denom             string
	TopN              int
	ValidatorSetCap   int
	ValidatorPowerCap int
	AllowInactiveVals bool
	MinStake          uint64
	Allowlist         []string
	Denylist          []string
	spec              *interchaintest.ChainSpec
}

var (
	// icsPath = "ics-path"
	// cd testing/consumer && DOCKER_BUILDKIT=0 docker build . --tag consumer:local
	ConsumerTestingChainSpec = interchaintest.ChainSpec{
		Name:          "ics-consumer",
		Version:       "v6.2.0",
		NumValidators: &vals, NumFullNodes: &fNodes,
		ChainConfig: ibc.ChainConfig{
			GasAdjustment:  3.0,
			TrustingPeriod: "504h",
			Type:           "cosmos",
			// Name:           "consumer",
			ChainID: "ics-consumer-1", // chainID := fmt.Sprintf("%s-%d", config.ChainName, len(p.Consumers)+1)
			// Bin:          "interchain-security-cdd", // consumer daemon democracy has staking, thus allowing proper genutils
			Denom:        "stake",
			Bech32Prefix: "consumer",
			// GasPrices:      "0.0" + "utoken",
			InterchainSecurityConfig: ibc.ICSConfig{
				ProviderVerOverride: "v4.1.0", // semver.Compare(p.GetNode().ICSVersion(ctx), "v4.1.0") > 0 && config.spec.InterchainSecurityConfig.ProviderVerOverride == ""
				// ConsumerVerOverride: "v0.0.0",
			},
			ModifyGenesis: cosmos.ModifyGenesis([]cosmos.GenesisKV{
				cosmos.NewGenesisKV("app_state.gov.params.voting_period", VotingPeriod),
				cosmos.NewGenesisKV("app_state.gov.params.max_deposit_period", MaxDepositPeriod),
				cosmos.NewGenesisKV("app_state.gov.params.min_deposit.0.denom", Denom),
				cosmos.NewGenesisKV("app_state.gov.params.min_deposit.0.amount", "1"),
				cosmos.NewGenesisKV("app_state.ccvconsumer.params.soft_opt_out_threshold", "0.0"), // if config.TopN >= 0
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
		// &ConsumerTestingChain,
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	provider := chains[0].(*cosmos.CosmosChain)
	// consumer := chains[1].(*cosmos.CosmosChain)

	// Relayer Factory
	r := interchaintest.NewBuiltinRelayerFactory(
		ibc.CosmosRly,
		zaptest.NewLogger(t),
		relayer.CustomDockerImage(RelayerRepo, RelayerVersion, "100:1000"),
		relayer.StartupFlags("--block-history", "200"),
	).Build(t, client, network)

	// Setup Interchain
	ic := interchaintest.NewInterchain().
		AddChain(provider)
	// AddChain(consumer).
	// AddRelayer(r, "rly") // will do later
	// AddProviderConsumerLink(interchaintest.ProviderConsumerLink{
	// Provider: provider,
	// Consumer: consumer,
	// Relayer:  r,
	// Path:     icsPath,
	// })

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

	// Add consumer chain after the startup
	spawnTime := time.Now().Add(time.Second * 10)
	chainID := ConsumerTestingChainSpec.ChainConfig.ChainID

	consumerCfg := ConsumerConfig{
		ChainName:         ConsumerTestingChainSpec.Name,
		Version:           ConsumerTestingChainSpec.Version,
		Denom:             ConsumerTestingChainSpec.Denom,
		TopN:              100,
		AllowInactiveVals: true,
		MinStake:          1_000_000,
		spec:              &ConsumerTestingChainSpec,
	}

	err = CreateConsumerPermissionless(ctx, provider, chainID, consumerCfg, spawnTime)
	require.NoError(t, err)

	cf = interchaintest.NewBuiltinChainFactory(
		zaptest.NewLogger(t),
		[]*interchaintest.ChainSpec{consumerCfg.spec},
	)
	chains2, err := cf.Chains(provider.GetNode().TestName)
	require.NoError(t, err)

	// We can't use AddProviderConsumerLink here because the provider chain is already built; we'll have to do everything by hand.
	cosmosConsumer := chains2[0].(*cosmos.CosmosChain)
	// consumers := []*cosmos.CosmosChain{cosmosConsumer}

	relayerWallet, err := cosmosConsumer.BuildRelayerWallet(ctx, "relayer-"+chainID)
	require.NoError(t, err)

	wallets := make([]ibc.Wallet, len(provider.Validators)+1)
	wallets[0] = relayerWallet
	// This is a hack, but we need to create wallets for the validators that have the right moniker.
	for i := 1; i <= len(provider.Validators); i++ {
		wallets[i], err = cosmosConsumer.BuildRelayerWallet(ctx, "validator") // hardcoded in ICT
		require.NoError(t, err)
	}
	walletAmounts := make([]ibc.WalletAmount, len(wallets))
	for i, wallet := range wallets {
		walletAmounts[i] = ibc.WalletAmount{
			Address: wallet.FormattedAddress(),
			Denom:   cosmosConsumer.Config().Denom,
			Amount:  sdkmath.NewInt(11_000_000_000), // gaia release/v21.x ValidatorFunds @7c0c4cbb7b26c48b235f60532223a74f4f68b830
		}
	}
	ic = interchaintest.NewInterchain().
		AddChain(cosmosConsumer, walletAmounts...).
		AddRelayer(r, "relayer")

	err = ic.Build(ctx, eRep, interchaintest.InterchainBuildOptions{
		Client:    client,
		NetworkID: network,
		TestName:  provider.GetNode().TestName,
	})
	require.NoError(t, err)

	// setup chain keys, stop & start relayer, then connect provider <> consumer
	for i, val := range cosmosConsumer.Validators {
		err := val.RecoverKey(ctx, "validator", wallets[i+1].Mnemonic())
		require.NoError(t, err)
	}
	// consumer, err := chainFromCosmosChain(cosmosConsumer, relayerWallet)
	// if err != nil {
	// 	return nil, err
	// }
	// err = relayer.SetupChainKeys(ctx, consumer)
	require.NoError(t, r.StopRelayer(ctx, eRep))
	require.NoError(t, r.StartRelayer(ctx, eRep))
	// connectProviderConsumer(ctx, p, consumer, relayer)

	// manual stuff

	/// ---

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

func CreateConsumerPermissionless(ctx context.Context, p *cosmos.CosmosChain, chainID string, config ConsumerConfig, spawnTime time.Time) error {
	initParams := &providertypes.ConsumerInitializationParameters{
		InitialHeight:                     clienttypes.Height{RevisionNumber: clienttypes.ParseChainID(chainID), RevisionHeight: 1},
		SpawnTime:                         spawnTime,
		BlocksPerDistributionTransmission: 1000,
		CcvTimeoutPeriod:                  2419200000000000,
		TransferTimeoutPeriod:             3600000000000,
		ConsumerRedistributionFraction:    "0.75",
		HistoricalEntries:                 10000,
		UnbondingPeriod:                   1728000000000000,
		GenesisHash:                       []byte("Z2VuX2hhc2g="),
		BinaryHash:                        []byte("YmluX2hhc2g="),
	}
	powerShapingParams := &providertypes.PowerShapingParameters{
		Top_N:              0,
		ValidatorSetCap:    uint32(config.ValidatorSetCap),
		ValidatorsPowerCap: uint32(config.ValidatorPowerCap),
		AllowInactiveVals:  config.AllowInactiveVals,
		MinStake:           config.MinStake,
		Allowlist:          config.Allowlist,
		Denylist:           config.Denylist,
	}
	params := providertypes.MsgCreateConsumer{
		ChainId: chainID,
		Metadata: providertypes.ConsumerMetadata{
			Name:        config.ChainName,
			Description: "Consumer chain",
			Metadata:    "ipfs://",
		},
		InitializationParameters: initParams,
		PowerShapingParameters:   powerShapingParams,
	}

	paramsBz, err := json.Marshal(params)
	if err != nil {
		return err
	}
	err = p.GetNode().WriteFile(ctx, paramsBz, "consumer-addition.json")
	if err != nil {
		return err
	}
	_, err = p.GetNode().ExecTx(ctx, interchaintest.FaucetAccountKeyName, "provider", "create-consumer", path.Join(p.GetNode().HomeDir(), "consumer-addition.json"))
	if err != nil {
		return err
	}
	if config.TopN > 0 {
		govAddress, err := p.GetGovernanceAddress(ctx)
		if err != nil {
			return err
		}
		consumerID, err := QueryJSON(ctx, p, fmt.Sprintf("chains.#(chain_id=%q).consumer_id", chainID), "provider", "list-consumer-chains")
		if err != nil {
			return err
		}
		update := &providertypes.MsgUpdateConsumer{
			ConsumerId:      consumerID.String(),
			NewOwnerAddress: govAddress,
			Metadata: &providertypes.ConsumerMetadata{
				Name:        config.ChainName,
				Description: "Consumer chain",
				Metadata:    "ipfs://",
			},
			InitializationParameters: initParams,
			PowerShapingParameters:   powerShapingParams,
		}
		updateBz, err := json.Marshal(update)
		if err != nil {
			return err
		}
		err = p.GetNode().WriteFile(ctx, updateBz, "consumer-update.json")
		if err != nil {
			return err
		}
		_, err = p.GetNode().ExecTx(ctx, interchaintest.FaucetAccountKeyName, "provider", "update-consumer", path.Join(p.GetNode().HomeDir(), "consumer-update.json"))
		if err != nil {
			return err
		}
		powerShapingParams.Top_N = uint32(config.TopN)
		update = &providertypes.MsgUpdateConsumer{
			Owner:      govAddress,
			ConsumerId: consumerID.String(),
			Metadata: &providertypes.ConsumerMetadata{
				Name:        config.ChainName,
				Description: "Consumer chain",
				Metadata:    "ipfs://",
			},
			InitializationParameters: initParams,
			PowerShapingParameters:   powerShapingParams,
		}
		prop, err := p.BuildProposal([]cosmos.ProtoMessage{update}, "update consumer", "update consumer", "", "5000000"+p.Config().Denom, "", false)
		if err != nil {
			return err
		}
		txhash, err := p.GetNode().SubmitProposal(ctx, "validator", prop)
		if err != nil {
			return err
		}
		propID, err := GetProposalID(ctx, p, txhash)
		if err != nil {
			return err
		}
		if err := PassProposal(ctx, p, propID); err != nil {
			return err
		}
	}
	return nil
}

func QueryJSON(ctx context.Context, c *cosmos.CosmosChain, jsonPath string, query ...string) (gjson.Result, error) {
	stdout, _, err := c.GetNode().ExecQuery(ctx, query...)
	if err != nil {
		return gjson.Result{}, err
	}
	retval := gjson.GetBytes(stdout, jsonPath)
	if !retval.Exists() {
		return gjson.Result{}, fmt.Errorf("json path %s not found in query result %s", jsonPath, stdout)
	}
	return retval, nil
}

// GetProposalID parses the proposal ID from the tx; necessary when the proposal type isn't accessible to interchaintest yet
func GetProposalID(ctx context.Context, c *cosmos.CosmosChain, txhash string) (string, error) {
	stdout, _, err := c.GetNode().ExecQuery(ctx, "tx", txhash)
	if err != nil {
		return "", err
	}
	result := struct {
		Events []abcitypes.Event `json:"events"`
	}{}
	if err := json.Unmarshal(stdout, &result); err != nil {
		return "", err
	}
	for _, event := range result.Events {
		if event.Type == "submit_proposal" {
			for _, attr := range event.Attributes {
				if string(attr.Key) == "proposal_id" {
					return string(attr.Value), nil
				}
			}
		}
	}
	return "", fmt.Errorf("proposal ID not found in tx %s", txhash)
}

func PassProposal(ctx context.Context, c *cosmos.CosmosChain, proposalID string) error {
	propID, err := strconv.ParseInt(proposalID, 10, 64)
	if err != nil {
		return err
	}
	err = c.VoteOnProposalAllValidators(ctx, uint64(propID), cosmos.ProposalVoteYes)
	if err != nil {
		return err
	}
	return WaitForProposalStatus(ctx, c, proposalID, govv1.StatusPassed)
}

func WaitForProposalStatus(ctx context.Context, c *cosmos.CosmosChain, proposalID string, status govv1.ProposalStatus) error {
	propID, err := strconv.ParseInt(proposalID, 10, 64)
	if err != nil {
		return err
	}
	chainHeight, err := c.Height(ctx)
	if err != nil {
		return err
	}
	// At 4s per block, 75 blocks is about 5 minutes.
	maxHeight := chainHeight + 75
	_, err = cosmos.PollForProposalStatusV1(ctx, c, chainHeight, maxHeight, uint64(propID), status)
	return err
}
