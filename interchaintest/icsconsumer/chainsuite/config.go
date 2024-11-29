package chainsuite

import (
	"time"

	"github.com/strangelove-ventures/interchaintest/v8"

	sdkmath "cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/types"
	e2e "github.com/lightlabs-dev/prysm/interchaintest"
)

type ChainScope int

const (
	ChainScopeSuite ChainScope = iota
	ChainScopeTest  ChainScope = iota
)

type SuiteConfig struct {
	ChainSpec      *interchaintest.ChainSpec
	UpgradeOnSetup bool
	CreateRelayer  bool
	Scope          ChainScope
}

var (
	Name             = e2e.Name
	Uatom            = e2e.Denom
	GovDepositAmount = "5000000" + Uatom
	GasPrices        = "0.005" + Uatom
)

const (
	Ucon                   = "ucon"
	NeutronDenom           = "untn"
	StrideDenom            = "ustr"
	DowntimeJailDuration   = 10 * time.Second
	ProviderSlashingWindow = 10
	// ValidatorCount         = 1
	UpgradeDelta           = 30
	ValidatorFunds         = 11_000_000_000
	ChainSpawnWait         = 155 * time.Second
	SlashingWindowConsumer = 20
	BlocksPerDistribution  = 10
	TransferPortID         = "transfer"
	// This is needed because not every ics image is in the default heighliner registry
	HyphaICSRepo = "ghcr.io/hyphacoop/ics"
	ICSUidGuid   = "1025:1025"
)

// These have to be vars so we can take their address
var (
	OneValidator  int = 1
	TwoValidators int = 2
	SixValidators int = 6
)

func MergeChainSpecs(spec, other *interchaintest.ChainSpec) *interchaintest.ChainSpec {
	if spec == nil {
		return other
	}
	if other == nil {
		return spec
	}
	spec.ChainConfig = spec.MergeChainSpecConfig(other.ChainConfig)
	if other.Name != "" {
		spec.Name = other.Name
	}
	if other.ChainName != "" {
		spec.ChainName = other.ChainName
	}
	if other.Version != "" {
		spec.Version = other.Version
	}
	if other.NoHostMount != nil {
		spec.NoHostMount = other.NoHostMount
	}
	if other.NumValidators != nil {
		spec.NumValidators = other.NumValidators
	}
	if other.NumFullNodes != nil {
		spec.NumFullNodes = other.NumFullNodes
	}
	return spec
}

func (c SuiteConfig) Merge(other SuiteConfig) SuiteConfig {
	c.ChainSpec = MergeChainSpecs(c.ChainSpec, other.ChainSpec)
	c.UpgradeOnSetup = other.UpgradeOnSetup
	c.CreateRelayer = other.CreateRelayer
	c.Scope = other.Scope
	return c
}

func DefaultGenesisAmounts(denom string) func(i int) (types.Coin, types.Coin) {
	return func(i int) (types.Coin, types.Coin) {
		if i >= SixValidators {
			panic("your chain has too many validators")
		}
		return types.Coin{
				Denom:  denom,
				Amount: sdkmath.NewInt(ValidatorFunds),
			}, types.Coin{
				Denom: denom,
				Amount: sdkmath.NewInt([]int64{
					30_000_000,
					29_000_000,
					20_000_000,
					10_000_000,
					7_000_000,
					4_000_000,
				}[i]),
			}
	}
}

func DefaultChainSpec(env Environment) *interchaintest.ChainSpec {
	// fullNodes := 0
	// var repository string
	// if env.DockerRegistry == "" {
	// 	repository = env.ImageName
	// } else {
	// 	repository = fmt.Sprintf("%s/%s", env.DockerRegistry, env.ImageName)
	// }
	// return &interchaintest.ChainSpec{
	// 	Name:          Name,
	// 	ChainName:     Name,
	// 	NumFullNodes:  &fullNodes,
	// 	NumValidators: &OneValidator,
	// 	Version:       env.OldGaiaImageVersion,
	// 	ChainConfig: ibc.ChainConfig{
	// 		Type:          "cosmos",
	// 		Name:          Name,
	// 		ChainID:       e2e.ChainID,
	// 		Bin:           e2e.Binary,
	// 		Bech32Prefix:  "prysm",
	// 		Denom:         Uatom,
	// 		GasPrices:     GasPrices,
	// 		GasAdjustment: 2.0,
	// 		ConfigFileOverrides: map[string]any{
	// 			"config/config.toml": DefaultConfigToml(),
	// 		},
	// 		Images: []ibc.DockerImage{{
	// 			Repository: repository,
	// 			UidGid:     "1025:1025", // this is the user in heighliner docker images
	// 		}},
	// 		ModifyGenesis:        cosmos.ModifyGenesis(DefaultGenesis()),
	// 		ModifyGenesisAmounts: DefaultGenesisAmounts(Uatom),
	// 	},
	// }
	return &e2e.DefaultChainSpec
}

func DefaultSuiteConfig(env Environment) SuiteConfig {
	return SuiteConfig{
		ChainSpec: DefaultChainSpec(env),
	}
}
