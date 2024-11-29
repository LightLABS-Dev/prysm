package e2e

import (
	"testing"

	"github.com/lightlabs-dev/prysm/interchaintest/chainsuite"
	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/stretchr/testify/suite"
)

type ConsumerLaunchSuite struct {
	*chainsuite.Suite
	OtherChain                   string
	OtherChainVersionPreUpgrade  string
	OtherChainVersionPostUpgrade string
	ShouldCopyProviderKey        []bool
}

func TestICS6ConsumerNoKeysChainLaunch(t *testing.T) {
	s := &ConsumerLaunchSuite{
		Suite: chainsuite.NewSuite(chainsuite.SuiteConfig{
			CreateRelayer: true,
			ChainSpec: &interchaintest.ChainSpec{
				NumValidators: &chainsuite.SixValidators,
				Name:          "prysm",
				Version:       "local",
				ChainConfig:   DefaultChainConfig,
			},
		}),
		OtherChain:                   "ics-consumer",
		OtherChainVersionPreUpgrade:  "v6.2.1", // or v6.0.0
		OtherChainVersionPostUpgrade: "v6.2.1",
		ShouldCopyProviderKey:        noProviderKeysCopied(),
	}
	suite.Run(t, s)
}

func noProviderKeysCopied() []bool {
	return []bool{false, false, false, false, false, false}
}

// func allProviderKeysCopied() []bool {
// 	return []bool{true, true, true, true, true, true}
// }

// func someProviderKeysCopied() []bool {
// 	return []bool{true, false, true, false, true, false}
// }

var (
	// TODO: this may no longer be required
	// icsPath = "ics-path"
	// cd testing/consumer && DOCKER_BUILDKIT=0 docker build . --tag consumer:local
	ConsumerTestingChainSpec = interchaintest.ChainSpec{
		Name:          "consumer",
		Version:       "local",
		NumValidators: &vals, NumFullNodes: &fNodes,
		ChainConfig: ibc.ChainConfig{
			GasAdjustment:  3.0,
			TrustingPeriod: "504h",
			Type:           "cosmos",
			Name:           "consumer",
			ChainID:        "icsconsumer-1", // chainID := fmt.Sprintf("%s-%d", config.ChainName, len(p.Consumers)+1)
			Bin:            "consumerd",     // consumer daemon democracy has staking, thus allowing proper genutils (?) maybe not
			Denom:          "stake",
			Bech32Prefix:   "cosmos",
			GasPrices:      "0.0" + "stake",
			InterchainSecurityConfig: ibc.ICSConfig{
				ProviderVerOverride: "v4.1.0", // semver.Compare(p.GetNode().ICSVersion(ctx), "v4.1.0") > 0 && config.spec.InterchainSecurityConfig.ProviderVerOverride == ""
				// ConsumerVerOverride: "v0.0.0",
			},
			Images: []ibc.DockerImage{
				ibc.NewDockerImage("consumer", "local", "1025:1025"),
			},
			CoinType:       "118",
			EncodingConfig: GetEncodingConfig(),
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

func (s *ConsumerLaunchSuite) TestChainLaunch() {
	cfg := chainsuite.ConsumerConfig{
		ChainName:             s.OtherChain,
		Version:               s.OtherChainVersionPreUpgrade,
		ShouldCopyProviderKey: s.ShouldCopyProviderKey,
		Denom:                 chainsuite.Ucon,
		TopN:                  94,
		Spec: &interchaintest.ChainSpec{
			ChainConfig: ibc.ChainConfig{
				Images: []ibc.DockerImage{
					{
						Repository: chainsuite.HyphaICSRepo,
						Version:    s.OtherChainVersionPreUpgrade,
						UIDGID:     chainsuite.ICSUidGuid,
					},
				},
			},
		},
	}
	consumer, err := s.Chain.AddConsumerChain(s.GetContext(), s.Relayer, cfg)
	s.Require().NoError(err)
	err = s.Chain.CheckCCV(s.GetContext(), consumer, s.Relayer, 1_000_000, 0, 1)
	s.Require().NoError(err)

	s.UpgradeChain()

	err = s.Chain.CheckCCV(s.GetContext(), consumer, s.Relayer, 1_000_000, 0, 1)
	s.Require().NoError(err)
	s.Require().NoError(chainsuite.SendSimpleIBCTx(s.GetContext(), s.Chain, consumer, s.Relayer))

	jailed, err := s.Chain.IsValidatorJailedForConsumerDowntime(s.GetContext(), s.Relayer, consumer, 1)
	s.Require().NoError(err)
	s.Require().True(jailed, "validator 1 should be jailed for downtime")
	jailed, err = s.Chain.IsValidatorJailedForConsumerDowntime(s.GetContext(), s.Relayer, consumer, 5)
	s.Require().NoError(err)
	s.Require().False(jailed, "validator 5 should not be jailed for downtime")

	cfg.Version = s.OtherChainVersionPostUpgrade
	cfg.Spec.ChainConfig.Images[0].Version = s.OtherChainVersionPostUpgrade
	consumer2, err := s.Chain.AddConsumerChain(s.GetContext(), s.Relayer, cfg)
	s.Require().NoError(err)
	err = s.Chain.CheckCCV(s.GetContext(), consumer2, s.Relayer, 1_000_000, 0, 1)
	s.Require().NoError(err)
	s.Require().NoError(chainsuite.SendSimpleIBCTx(s.GetContext(), s.Chain, consumer2, s.Relayer))

	jailed, err = s.Chain.IsValidatorJailedForConsumerDowntime(s.GetContext(), s.Relayer, consumer2, 1)
	s.Require().NoError(err)
	s.Require().True(jailed, "validator 1 should be jailed for downtime")
	jailed, err = s.Chain.IsValidatorJailedForConsumerDowntime(s.GetContext(), s.Relayer, consumer2, 5)
	s.Require().NoError(err)
	s.Require().False(jailed, "validator 5 should not be jailed for downtime")
}
