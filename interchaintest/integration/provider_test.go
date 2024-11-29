package e2e

import (
	"testing"
	"time"

	"github.com/lightlabs-dev/prysm/interchaintest/integration/chainsuite"

	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	providertypes "github.com/cosmos/interchain-security/v6/x/ccv/provider/types"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/testutil"

	"github.com/stretchr/testify/suite"
)

func TestICSProviderSuite(t *testing.T) {
	s := &ProviderSuite{}

	suite.Run(t, s)
}

////////////////////////////////////////////////////////////
//				Chain CRUD flow test					  //
////////////////////////////////////////////////////////////

// Test Creating a chain (MsgCreateConsumer)
// Confirm that a chain can be created with the minimum params (only consumer metadata)
// Confirm that a chain can be created with all params
// Confirm that a chain can be created with initialization parameters that do not contain a spawn time
// Confirm that if there are no opted-in validators at spawn time, the chain fails to launch and moves back to its Registered phase having reset its spawn time
func (s *ProviderSuite) TestProviderCreateConsumer() {
	testAcc := s.Provider.TestWallets[0].FormattedAddress()
	testAccKey := s.Provider.TestWallets[0].KeyName()

	// Confirm that a chain can be create with the minimum params (metadata)
	chainName := "minParamAddConsumer-1"
	createConsumerMsg := msgCreateConsumer(chainName, nil, nil, testAcc)
	consumerId, err := s.Provider.CreateConsumer(s.GetContext(), createConsumerMsg, testAccKey)
	s.Require().NoError(err)
	consumerChain, err := s.Provider.GetConsumerChain(s.GetContext(), consumerId)
	s.Require().NoError(err)
	s.Require().Equal(providertypes.CONSUMER_PHASE_REGISTERED.String(), consumerChain.Phase)

	// Confirm that a chain can be created with initialization parameters that do not contain a spawn time
	chainName = "noSpawnTimeAddConsumer-0"
	powerShapingParams := powerShapingParamsTemplate()
	createConsumerMsg = msgCreateConsumer(chainName, consumerInitParamsTemplate(nil), powerShapingParams, testAcc)
	consumerId, err = s.Provider.CreateConsumer(s.GetContext(), createConsumerMsg, testAccKey)
	s.Require().NoError(err)
	consumerChain, err = s.Provider.GetConsumerChain(s.GetContext(), consumerId)
	s.Require().NoError(err)
	s.Require().Equal(providertypes.CONSUMER_PHASE_REGISTERED.String(), consumerChain.Phase)

	// Confirm that a chain can be created with all params(future spawn time)
	valConsAddr, err := s.Provider.GetValidatorConsAddress(s.GetContext(), 0)
	s.Require().NoError(err)
	powerShapingParams.Allowlist = []string{valConsAddr}
	powerShapingParams.Denylist = []string{"prysmvalcons1l9qq4m300z8c5ez86ak2mp8znftewkwgu2dnj6"}

	chainName = "allParamsFutureSpawnTimeAddConsumer"
	spawnTimeFromNow := time.Now().Add(time.Hour)
	createConsumerMsg = msgCreateConsumer(chainName, consumerInitParamsTemplate(&spawnTimeFromNow), powerShapingParams, testAcc)
	consumerId, err = s.Provider.CreateConsumer(s.GetContext(), createConsumerMsg, testAccKey)
	s.Require().NoError(err)
	consumerChain, err = s.Provider.GetConsumerChain(s.GetContext(), consumerId)
	s.Require().NoError(err)
	s.Require().Equal(providertypes.CONSUMER_PHASE_INITIALIZED.String(), consumerChain.Phase)

	// Confirm that a chain can be created with all params(past spawn time)
	chainName = "allParamsPastSpawnTimeAddConsumer"
	spawnTimeFromNow = time.Now().Add(-time.Hour)
	createConsumerMsg = msgCreateConsumer(chainName, consumerInitParamsTemplate(&spawnTimeFromNow), powerShapingParams, testAcc)
	consumerId, err = s.Provider.CreateConsumer(s.GetContext(), createConsumerMsg, testAccKey)
	s.Require().NoError(err)
	consumerChain, err = s.Provider.GetConsumerChain(s.GetContext(), consumerId)
	s.Require().NoError(err)
	// spawn time is in the past but there are no opted-in validators
	s.Require().Equal(providertypes.CONSUMER_PHASE_REGISTERED.String(), consumerChain.Phase)
}

// Test Creating a chain (MsgCreateConsumer)
// Confirm that a chain with TopN > 0 is rejected
// Confirm that a chain without the minimum params (metadata) is rejected
// Confirm that a chain voted 'no' is rejected
func (s *ProviderSuite) TestProviderCreateConsumerRejection() {
	testAcc := s.Provider.TestWallets[1].FormattedAddress()
	testAccKey := s.Provider.TestWallets[1].KeyName()

	chainName := "rejectConsumer"
	// Confirm that a chain with TopN > 0 is rejected
	createConsumerMsg := msgCreateConsumer(chainName, nil, powerShapingParamsTemplate(), testAcc)
	createConsumerMsg.PowerShapingParameters.Top_N = 100
	_, err := s.Provider.CreateConsumer(s.GetContext(), createConsumerMsg, testAccKey)
	s.Require().Error(err)

	// Confirm that a chain without the minimum params (metadata) is rejected
	createConsumerMsg = msgCreateConsumer(chainName, nil, nil, testAcc)
	createConsumerMsg.Metadata = providertypes.ConsumerMetadata{}
	_, err = s.Provider.CreateConsumer(s.GetContext(), createConsumerMsg, testAccKey)
	s.Require().Error(err)
}

// Test Opting in validators to a chain (MsgOptIn)
// Confirm that a chain can be created and validators can be opted in
// Scenario 1: Validators opted in, MsgUpdateConsumer called to set spawn time in the past -> chain should start.
// Scenario 2: Validators opted in, spawn time is in the future, the chain starts after the spawn time.
func (s *ProviderSuite) TestProviderValidatorOptIn() {
	testAcc := s.Provider.TestWallets[2].FormattedAddress()
	testAccKey := s.Provider.TestWallets[2].KeyName()

	// Scenario 1: Validators opted in, MsgUpdateConsumer called to set spawn time in the past -> chain should start.
	chainName := "optInScenario1"
	spawnTime := time.Now().Add(time.Hour)
	consumerInitParams := consumerInitParamsTemplate(&spawnTime)
	createConsumerMsg := msgCreateConsumer(chainName, consumerInitParams, powerShapingParamsTemplate(), testAcc)
	consumerId, err := s.Provider.CreateConsumer(s.GetContext(), createConsumerMsg, testAccKey)
	s.Require().NoError(err)
	consumerChain, err := s.Provider.GetConsumerChain(s.GetContext(), consumerId)
	s.Require().NoError(err)
	s.Require().NoError(s.Provider.OptIn(s.GetContext(), consumerChain.ConsumerID, 0))
	consumerInitParams.SpawnTime = time.Now()
	upgradeMsg := &providertypes.MsgUpdateConsumer{
		Owner:                    testAcc,
		ConsumerId:               consumerChain.ConsumerID,
		NewOwnerAddress:          testAcc,
		InitializationParameters: consumerInitParams,
		PowerShapingParameters:   powerShapingParamsTemplate(),
	}
	s.Require().NoError(s.Provider.UpdateConsumer(s.GetContext(), upgradeMsg, testAccKey))
	s.Require().NoError(testutil.WaitForBlocks(s.GetContext(), 1, s.Provider))
	consumerChain, err = s.Provider.GetConsumerChain(s.GetContext(), consumerId)
	s.Require().NoError(err)
	// chain is started
	s.Require().Equal(providertypes.CONSUMER_PHASE_LAUNCHED.String(), consumerChain.Phase)

	// Scenario 2: Validators opted in, spawn time is in the future, the chain should not start before the spawn time.
	chainName = "optInScenario2"
	spawnTime = time.Now().Add(30 * time.Second)
	consumerInitParams = consumerInitParamsTemplate(&spawnTime)
	createConsumerMsg = msgCreateConsumer(chainName, consumerInitParams, powerShapingParamsTemplate(), testAcc)
	consumerId, err = s.Provider.CreateConsumer(s.GetContext(), createConsumerMsg, testAccKey)
	s.Require().NoError(err)
	consumerChain, err = s.Provider.GetConsumerChain(s.GetContext(), consumerId)
	s.Require().NoError(err)
	s.Require().NoError(s.Provider.OptIn(s.GetContext(), consumerChain.ConsumerID, 0))
	consumerChain, err = s.Provider.GetConsumerChain(s.GetContext(), consumerId)
	s.Require().NoError(err)
	s.Require().Equal(providertypes.CONSUMER_PHASE_INITIALIZED.String(), consumerChain.Phase)
	time.Sleep(time.Until(consumerInitParams.SpawnTime))
	s.Require().NoError(testutil.WaitForBlocks(s.GetContext(), 2, s.Provider))
	// chain is started after spawn time
	consumerChain, err = s.Provider.GetConsumerChain(s.GetContext(), consumerId)
	s.Require().NoError(err)
	s.Require().Equal(providertypes.CONSUMER_PHASE_LAUNCHED.String(), consumerChain.Phase)
}

// Test Updating a chain (MsgUpdateConsumer)
// Confirm that a chain can update a combination of the metadata, initialization, and power-shaping parameters
// If there are no opted-in validators and the spawn time is in the past, the chain should not start.
// Confirm that a chain remains in the Registered phase unless all the initialization parameters are set for it
func (s *ProviderSuite) TestProviderUpdateConsumer() {
	testAcc := s.Provider.TestWallets[4].FormattedAddress()
	testAccKey := s.Provider.TestWallets[4].KeyName()

	chainName := "updateConsumer"
	spawnTime := time.Now().Add(-time.Hour)
	initParams := consumerInitParamsTemplate(&spawnTime)
	powerShapingParams := powerShapingParamsTemplate()

	// create consumer
	createConsumerMsg := msgCreateConsumer(chainName, initParams, powerShapingParams, testAcc)
	consumerId, err := s.Provider.CreateConsumer(s.GetContext(), createConsumerMsg, testAccKey)
	s.Require().NoError(err)
	consumerChain, err := s.Provider.GetConsumerChain(s.GetContext(), consumerId)
	s.Require().NoError(err)
	s.Require().NoError(s.Provider.OptIn(s.GetContext(), consumerChain.ConsumerID, 0))
	s.Require().Equal(providertypes.CONSUMER_PHASE_REGISTERED.String(), consumerChain.Phase)

	// updated consumer with the minimum params (metadata) - regeistered phase
	upgradeMsg := &providertypes.MsgUpdateConsumer{
		Owner:                    testAcc,
		ConsumerId:               consumerChain.ConsumerID,
		NewOwnerAddress:          testAcc,
		InitializationParameters: nil,
		PowerShapingParameters:   nil,
	}
	s.Require().NoError(s.Provider.UpdateConsumer(s.GetContext(), upgradeMsg, testAccKey))
	s.Require().NoError(testutil.WaitForBlocks(s.GetContext(), 1, s.Provider))
	consumerChain, err = s.Provider.GetConsumerChain(s.GetContext(), consumerId)
	s.Require().NoError(err)
	s.Require().Equal(providertypes.CONSUMER_PHASE_REGISTERED.String(), consumerChain.Phase)

	// updated consumer with all params future timestamp - initialized phase
	valConsAddr, err := s.Provider.GetValidatorConsAddress(s.GetContext(), 0)
	s.Require().NoError(err)
	powerShapingParams.Allowlist = []string{valConsAddr}
	powerShapingParams.Denylist = []string{"prysmvalcons1l9qq4m300z8c5ez86ak2mp8znftewkwgu2dnj6"}
	initParams.SpawnTime = time.Now().Add(time.Hour)
	upgradeMsg.InitializationParameters = initParams
	upgradeMsg.PowerShapingParameters = powerShapingParams
	s.Require().NoError(s.Provider.UpdateConsumer(s.GetContext(), upgradeMsg, testAccKey))
	s.Require().NoError(testutil.WaitForBlocks(s.GetContext(), 1, s.Provider))
	consumerChain, err = s.Provider.GetConsumerChain(s.GetContext(), consumerId)
	s.Require().NoError(err)
	s.Require().Equal(providertypes.CONSUMER_PHASE_INITIALIZED.String(), consumerChain.Phase)

	// updated consumer with all params past timestamp - launched phase
	initParams.SpawnTime = time.Now()
	upgradeMsg.InitializationParameters = initParams
	s.Require().NoError(s.Provider.UpdateConsumer(s.GetContext(), upgradeMsg, testAccKey))
	s.Require().NoError(testutil.WaitForBlocks(s.GetContext(), 1, s.Provider))
	consumerChain, err = s.Provider.GetConsumerChain(s.GetContext(), consumerId)
	s.Require().NoError(err)
	// chain is started
	s.Require().Equal(providertypes.CONSUMER_PHASE_LAUNCHED.String(), consumerChain.Phase)
}

// Create a chain, opt-in validators, and transform the opt-in to TopN via `tx gov submit-proposal` using MsgUpdateConsumer
// Confirm that the chain starts successfully and is owned by governance
// Confirm that the chain can be updated to a lower TopN
// Confirm that the chain can be updated to a higher TopN
// Confirm that the owner of the chain cannot change as long as it remains a Top N chain
func (s *ProviderSuite) TestProviderTransformOptInToTopN() {
	testAcc := s.Provider.TestWallets[5].FormattedAddress()
	testAccKey := s.Provider.TestWallets[5].KeyName()

	// Create an opt-in chain, owner is testAcc1
	chainName := "transformOptinToTopNConsumer"
	spawnTime := time.Now().Add(time.Hour)
	initParams := consumerInitParamsTemplate(&spawnTime)
	powerShapingParams := powerShapingParamsTemplate()
	createConsumerMsg := msgCreateConsumer(chainName, initParams, powerShapingParams, testAcc)
	consumerId, err := s.Provider.CreateConsumer(s.GetContext(), createConsumerMsg, testAccKey)
	s.Require().NoError(err)
	consumerChain, err := s.Provider.GetConsumerChain(s.GetContext(), consumerId)
	s.Require().NoError(err)
	s.Require().Equal(0, consumerChain.PowerShapingParams.TopN)
	s.Require().NoError(s.Provider.OptIn(s.GetContext(), consumerId, 0))
	s.Require().Equal(testAcc, consumerChain.OwnerAddress)

	// Transform chain from opt-in to top N
	// transfer ownership
	upgradeMsg := &providertypes.MsgUpdateConsumer{
		Owner:           testAcc,
		ConsumerId:      consumerId,
		NewOwnerAddress: chainsuite.GovModuleAddress,
	}
	s.Require().NoError(s.Provider.UpdateConsumer(s.GetContext(), upgradeMsg, testAccKey))
	consumerChain, err = s.Provider.GetConsumerChain(s.GetContext(), consumerId)
	s.Require().NoError(err)
	s.Require().Equal(chainsuite.GovModuleAddress, consumerChain.OwnerAddress)
	// Confirm that the chain can be updated to a lower TopN
	spawTimeFromNow := 10 * time.Second
	initParams.SpawnTime = time.Now().Add(spawTimeFromNow)
	powerShapingParams.Top_N = 50
	upgradeMsg = &providertypes.MsgUpdateConsumer{
		Owner:                    chainsuite.GovModuleAddress,
		ConsumerId:               consumerId,
		NewOwnerAddress:          chainsuite.GovModuleAddress,
		InitializationParameters: initParams,
		PowerShapingParameters:   powerShapingParams,
	}
	s.Require().NoError(s.Provider.ExecuteProposalMsg(s.GetContext(), upgradeMsg, chainsuite.GovModuleAddress, chainName, cosmos.ProposalVoteYes, govv1.StatusPassed, false))
	time.Sleep(spawTimeFromNow)
	s.Require().NoError(testutil.WaitForBlocks(s.GetContext(), 1, s.Provider))
	updatedChain, err := s.Provider.GetConsumerChain(s.GetContext(), consumerId)
	s.Require().NoError(err)
	s.Require().Equal(providertypes.CONSUMER_PHASE_LAUNCHED.String(), updatedChain.Phase)
	s.Require().Equal(50, updatedChain.PowerShapingParams.TopN)

	//Confirm that the chain can be updated to a higher TopN
	powerShapingParams.Top_N = 100
	upgradeMsg = &providertypes.MsgUpdateConsumer{
		Owner:                  chainsuite.GovModuleAddress,
		ConsumerId:             consumerId,
		NewOwnerAddress:        chainsuite.GovModuleAddress,
		PowerShapingParameters: powerShapingParams,
	}
	s.Require().NoError(s.Provider.ExecuteProposalMsg(s.GetContext(), upgradeMsg, chainsuite.GovModuleAddress, chainName, cosmos.ProposalVoteYes, govv1.StatusPassed, false))
	updatedChain, err = s.Provider.GetConsumerChain(s.GetContext(), consumerId)
	s.Require().NoError(err)
	s.Require().Equal(providertypes.CONSUMER_PHASE_LAUNCHED.String(), updatedChain.Phase)
	s.Require().Equal(100, updatedChain.PowerShapingParams.TopN)

	// Confirm that the owner of the chain cannot change as long as it remains a Top N chain
	upgradeMsg = &providertypes.MsgUpdateConsumer{
		Owner:           chainsuite.GovModuleAddress,
		ConsumerId:      consumerId,
		NewOwnerAddress: testAcc,
	}
	s.Require().Error(s.Provider.ExecuteProposalMsg(s.GetContext(), upgradeMsg, chainsuite.GovModuleAddress, chainName, cosmos.ProposalVoteYes, govv1.StatusPassed, false))
}

// Create a Top N chain, and transform it to an opt-in via `tx gov submit-proposal` using MsgUpdateConsumer
// Confirm that the chain is now not owned by governance
func (s *ProviderSuite) TestProviderTransformTopNtoOptIn() {
	testAcc := s.Provider.TestWallets[6].FormattedAddress()

	chainName := "transformTopNtoOptIn"
	// create top N chain
	spawnTimeFromNow := time.Now().Add(time.Hour)
	powerShapingParams := powerShapingParamsTemplate()
	initParams := consumerInitParamsTemplate(&spawnTimeFromNow)
	proposalMsg := msgCreateConsumer(chainName, initParams, powerShapingParams, chainsuite.GovModuleAddress)
	s.Require().NoError(s.Provider.ExecuteProposalMsg(s.GetContext(), proposalMsg, chainsuite.GovModuleAddress, chainName, cosmos.ProposalVoteYes, govv1.StatusPassed, false))
	consumerChain, err := s.Provider.GetConsumerChainByChainId(s.GetContext(), chainName)
	s.Require().NoError(err)
	powerShapingParams.Top_N = 100
	initParams.SpawnTime = time.Now().Add(-time.Hour)
	upgradeMsg := &providertypes.MsgUpdateConsumer{
		Owner:                    chainsuite.GovModuleAddress,
		ConsumerId:               consumerChain.ConsumerID,
		NewOwnerAddress:          chainsuite.GovModuleAddress,
		PowerShapingParameters:   powerShapingParams,
		InitializationParameters: initParams,
	}
	s.Require().NoError(s.Provider.ExecuteProposalMsg(s.GetContext(), upgradeMsg, chainsuite.GovModuleAddress, chainName, cosmos.ProposalVoteYes, govv1.StatusPassed, false))
	consumerChain, err = s.Provider.GetConsumerChainByChainId(s.GetContext(), chainName)
	s.Require().NoError(err)
	s.Require().Equal(providertypes.CONSUMER_PHASE_LAUNCHED.String(), consumerChain.Phase)
	s.Require().Equal(powerShapingParams.Top_N, uint32(consumerChain.TopN))

	// Transform to opt in chain
	powerShapingParams.Top_N = 0
	upgradeMsg = &providertypes.MsgUpdateConsumer{
		Owner:                  chainsuite.GovModuleAddress,
		ConsumerId:             consumerChain.ConsumerID,
		NewOwnerAddress:        testAcc,
		PowerShapingParameters: powerShapingParams,
	}
	s.Require().NoError(s.Provider.ExecuteProposalMsg(s.GetContext(), upgradeMsg, chainsuite.GovModuleAddress, chainName, cosmos.ProposalVoteYes, govv1.StatusPassed, false))
	optInChain, err := s.Provider.GetConsumerChain(s.GetContext(), consumerChain.ConsumerID)
	s.Require().NoError(err)
	s.Require().Equal(powerShapingParams.Top_N, uint32(optInChain.PowerShapingParams.TopN))
	s.Require().Equal(testAcc, optInChain.OwnerAddress)
}

// TestOptOut tests removin validator from consumer-opted-in-validators
func (s *ProviderSuite) TestOptOut() {
	testAcc := s.Provider.TestWallets[7].FormattedAddress()
	testAccKey := s.Provider.TestWallets[7].KeyName()

	// Add consume chain
	chainName := "TestOptOut"
	spawnTime := time.Now().Add(time.Hour)
	consumerInitParams := consumerInitParamsTemplate(&spawnTime)
	powerShapingParams := powerShapingParamsTemplate()
	createConsumerMsg := msgCreateConsumer(chainName, consumerInitParams, powerShapingParams, testAcc)
	consumerId, err := s.Provider.CreateConsumer(s.GetContext(), createConsumerMsg, testAccKey)
	s.Require().NoError(err)
	consumerChain, err := s.Provider.GetConsumerChain(s.GetContext(), consumerId)
	s.Require().NoError(err)

	//OptIn
	s.Require().NoError(s.Provider.OptIn(s.GetContext(), consumerChain.ConsumerID, 0))
	consumerInitParams.SpawnTime = time.Now()
	upgradeMsg := &providertypes.MsgUpdateConsumer{
		Owner:                    testAcc,
		ConsumerId:               consumerChain.ConsumerID,
		NewOwnerAddress:          testAcc,
		InitializationParameters: consumerInitParams,
		PowerShapingParameters:   powerShapingParams,
	}
	s.Require().NoError(s.Provider.UpdateConsumer(s.GetContext(), upgradeMsg, testAccKey))
	s.Require().NoError(testutil.WaitForBlocks(s.GetContext(), 1, s.Provider))
	consumerChain, err = s.Provider.GetConsumerChain(s.GetContext(), consumerId)
	s.Require().NoError(err)
	s.Require().Equal(providertypes.CONSUMER_PHASE_LAUNCHED.String(), consumerChain.Phase)
	optInVals, err := s.Provider.GetOptInValidators(s.GetContext(), consumerChain.ConsumerID)
	s.Require().NoError(err)
	s.Require().Equal(1, len(optInVals.ValidatorsProviderAddresses))
	valConsAddr, err := s.Provider.GetValidatorConsAddress(s.GetContext(), 0)
	s.Require().NoError(err)
	s.Require().Equal(valConsAddr, optInVals.ValidatorsProviderAddresses[0])

	//OptOut
	s.Require().NoError(s.Provider.OptOut(s.GetContext(), consumerChain.ConsumerID, 0))
	s.Require().NoError(testutil.WaitForBlocks(s.GetContext(), 1, s.Provider))
	optInVals, err = s.Provider.GetOptInValidators(s.GetContext(), consumerChain.ConsumerID)
	s.Require().NoError(err)
	s.Require().Equal(0, len(optInVals.ValidatorsProviderAddresses))
}

// Test removing a chain (MsgRemoveConsumer)
// Confirm that the chain moves to the Stopped phase and is not getting any VSCPackets anymore
// Confirm that after unbonding period, the chain moves to the Deleted phase and things like consumer id to client id
// associations are deleted, but the chain metadata and the chain id are not deleted
func (s *ProviderSuite) TestProviderRemoveConsumer() {
	testAcc := s.Provider.TestWallets[8].FormattedAddress()
	testAccKey := s.Provider.TestWallets[8].KeyName()

	// Test removing a chain
	chainName := "removeConsumer"
	spawnTime := time.Now().Add(time.Hour)
	initParams := consumerInitParamsTemplate(&spawnTime)
	powerShapingParams := powerShapingParamsTemplate()
	createConsumerMsg := msgCreateConsumer(chainName, initParams, powerShapingParams, testAcc)
	consumerId, err := s.Provider.CreateConsumer(s.GetContext(), createConsumerMsg, testAccKey)
	s.Require().NoError(err)
	consumerChain, err := s.Provider.GetConsumerChain(s.GetContext(), consumerId)
	s.Require().NoError(err)
	s.Require().NoError(s.Provider.OptIn(s.GetContext(), consumerChain.ConsumerID, 0))

	// cannot be removed if not in phase CONSUMER_PHASE_LAUNCHED
	s.Require().Error(s.Provider.RemoveConsumer(s.GetContext(), consumerChain.ConsumerID, testAccKey))

	// update spawn time
	initParams.SpawnTime = time.Now()
	upgradeMsg := &providertypes.MsgUpdateConsumer{
		Owner:                    testAcc,
		ConsumerId:               consumerChain.ConsumerID,
		NewOwnerAddress:          testAcc,
		InitializationParameters: initParams,
		PowerShapingParameters:   powerShapingParams,
	}
	s.Require().NoError(s.Provider.UpdateConsumer(s.GetContext(), upgradeMsg, testAccKey))
	s.Require().NoError(testutil.WaitForBlocks(s.GetContext(), 1, s.Provider))
	chainToRemove, err := s.Provider.GetConsumerChain(s.GetContext(), consumerChain.ConsumerID)
	s.Require().NoError(err)
	s.Require().Equal(providertypes.CONSUMER_PHASE_LAUNCHED.String(), chainToRemove.Phase)
	// get consumer genesis data
	consumerGenesis, err := s.Provider.GetConsumerGenesis(s.GetContext(), consumerChain.ConsumerID)
	s.Require().NoError(err)
	s.Require().True(len(consumerGenesis.Provider.InitialValSet) == 1)

	// remove consumer successfully
	s.Require().NoError(s.Provider.RemoveConsumer(s.GetContext(), consumerChain.ConsumerID, testAccKey))
	chainToRemove, err = s.Provider.GetConsumerChain(s.GetContext(), consumerChain.ConsumerID)
	s.Require().NoError(err)
	s.Require().Equal(providertypes.CONSUMER_PHASE_STOPPED.String(), chainToRemove.Phase)
	time.Sleep(chainsuite.ProviderUnbondingTime)
	s.Require().NoError(testutil.WaitForBlocks(s.GetContext(), 1, s.Provider))
	chainToRemove, err = s.Provider.GetConsumerChain(s.GetContext(), consumerChain.ConsumerID)
	s.Require().NoError(err)
	s.Require().Equal(providertypes.CONSUMER_PHASE_DELETED.String(), chainToRemove.Phase)

	// consumer genesis data does not exist anymore
	_, err = s.Provider.GetConsumerGenesis(s.GetContext(), consumerChain.ConsumerID)
	s.Require().Error(err)
}
