package endtoend

import (
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/prysmaticlabs/prysm/config/params"
	ev "github.com/prysmaticlabs/prysm/testing/endtoend/evaluators"
	"github.com/prysmaticlabs/prysm/testing/endtoend/helpers"
	e2eParams "github.com/prysmaticlabs/prysm/testing/endtoend/params"
	"github.com/prysmaticlabs/prysm/testing/endtoend/types"
	"github.com/prysmaticlabs/prysm/testing/require"
)

type testArgs struct {
	usePrysmSh          bool
	useWeb3RemoteSigner bool
}

func TestEndToEnd_MinimalConfig(t *testing.T) {
	e2eMinimal(t, &testArgs{
		usePrysmSh:          false,
		useWeb3RemoteSigner: false,
	})
}

func TestEndToEnd_MinimalConfig_Web3Signer(t *testing.T) {
	e2eMinimal(t, &testArgs{
		usePrysmSh:          false,
		useWeb3RemoteSigner: true,
	})
}

// Run minimal e2e config with the current release validator against latest beacon node.
func TestEndToEnd_MinimalConfig_ValidatorAtCurrentRelease(t *testing.T) {
	e2eMinimal(t, &testArgs{
		usePrysmSh:          true,
		useWeb3RemoteSigner: false,
	})
}

func e2eMinimal(t *testing.T, args *testArgs) {
	params.UseE2EConfig()
	require.NoError(t, e2eParams.Init(e2eParams.StandardBeaconCount))

	// Run for 10 epochs if not in long-running to confirm long-running has no issues.
	var err error
	epochsToRun := 10
	epochStr, longRunning := os.LookupEnv("E2E_EPOCHS")
	if longRunning {
		epochsToRun, err = strconv.Atoi(epochStr)
		require.NoError(t, err)
	}
	// TODO(#10053): Web3signer does not support bellatrix yet.
	if args.useWeb3RemoteSigner {
		epochsToRun = helpers.BellatrixE2EForkEpoch - 1
	}
	if args.usePrysmSh {
		// If using prysm.sh, run for only 6 epochs.
		// TODO(#9166): remove this block once v2 changes are live.
		epochsToRun = helpers.AltairE2EForkEpoch - 1
	}
	seed := 0
	seedStr, isValid := os.LookupEnv("E2E_SEED")
	if isValid {
		seed, err = strconv.Atoi(seedStr)
		require.NoError(t, err)
	}
	tracingPort := e2eParams.TestParams.Ports.JaegerTracingPort
	tracingEndpoint := fmt.Sprintf("127.0.0.1:%d", tracingPort)
	evals := []types.Evaluator{
		ev.PeersConnect,
		ev.HealthzCheck,
		ev.MetricsCheck,
		ev.ValidatorsAreActive,
		ev.ValidatorsParticipatingAtEpoch(2),
		ev.FinalizationOccurs(3),
		ev.PeersCheck,
		ev.ProcessesDepositsInBlocks,
		ev.VerifyBlockGraffiti,
		ev.ActivatesDepositedValidators,
		ev.DepositedValidatorsAreActive,
		ev.ProposeVoluntaryExit,
		ev.ValidatorHasExited,
		ev.ValidatorsVoteWithTheMajority,
		ev.ColdStateCheckpoint,
		ev.AltairForkTransition,
		ev.BellatrixForkTransition,
		ev.APIMiddlewareVerifyIntegrity,
		ev.APIGatewayV1Alpha1VerifyIntegrity,
		ev.FinishedSyncing,
		ev.AllNodesHaveSameHead,
		ev.ValidatorSyncParticipation,
		ev.TransactionsPresent,
	}
	testConfig := &types.E2EConfig{
		BeaconFlags: []string{
			fmt.Sprintf("--slots-per-archive-point=%d", params.BeaconConfig().SlotsPerEpoch*16),
			fmt.Sprintf("--tracing-endpoint=http://%s", tracingEndpoint),
			"--enable-tracing",
			"--trace-sample-fraction=1.0",
		},
		ValidatorFlags:      []string{},
		EpochsToRun:         uint64(epochsToRun),
		TestSync:            true,
		TestFeature:         true,
		TestDeposits:        true,
		UsePrysmShValidator: args.usePrysmSh,
		UsePprof:            !longRunning,
		UseWeb3RemoteSigner: args.useWeb3RemoteSigner,
		TracingSinkEndpoint: tracingEndpoint,
		Evaluators:          evals,
		Seed:                int64(seed),
	}

	newTestRunner(t, testConfig).run()
}
