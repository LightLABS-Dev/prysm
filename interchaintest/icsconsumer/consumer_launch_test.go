package consumer_chain_test

import (
	"testing"

	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/stretchr/testify/suite"

	e2e "github.com/lightlabs-dev/prysm/interchaintest"
	"github.com/lightlabs-dev/prysm/interchaintest/icsconsumer/chainsuite"
)

type ConsumerLaunchSuite struct {
	*chainsuite.Suite
	OtherChain                   string
	OtherChainVersionPreUpgrade  string
	OtherChainVersionPostUpgrade string
	ShouldCopyProviderKey        []bool
}

func TestICS6Consumer(t *testing.T) {
	numVals := chainsuite.TwoValidators

	s := &ConsumerLaunchSuite{
		Suite: chainsuite.NewSuite(chainsuite.SuiteConfig{
			CreateRelayer: true,
			ChainSpec: &interchaintest.ChainSpec{
				NumValidators: &numVals,
				ChainConfig:   e2e.DefaultChainConfig,
			},
		}),
		OtherChain:                   "ics-consumer",
		OtherChainVersionPreUpgrade:  "v6.2.1",
		OtherChainVersionPostUpgrade: "v6.2.1",
		ShouldCopyProviderKey:        noProviderKeysCopied(numVals),
	}
	suite.Run(t, s)
}

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
						UidGid:     chainsuite.ICSUidGuid,
					},
				},
			},
		},
	}

	consumer, err := s.Chain.AddConsumerChain(s.GetContext(), s.Relayer, cfg)
	s.Require().NoError(err)
	err = s.Chain.CheckCCV(s.GetContext(), consumer, s.Relayer, 1_000_000, 0, 1)
	s.Require().NoError(err)

	// s.UpgradeChain() // no upgrade to test for now

	err = s.Chain.CheckCCV(s.GetContext(), consumer, s.Relayer, 1_000_000, 0, 1)
	s.Require().NoError(err)
	s.Require().NoError(chainsuite.SendSimpleIBCTx(s.GetContext(), s.Chain, consumer, s.Relayer))
}

func noProviderKeysCopied(numVals int) []bool {
	ret := make([]bool, numVals)
	for i := range ret {
		ret[i] = false
	}
	return ret
}
