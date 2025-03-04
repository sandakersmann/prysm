package blockchain

import (
	"context"
	"testing"

	testDB "github.com/prysmaticlabs/prysm/beacon-chain/db/testing"
	"github.com/prysmaticlabs/prysm/beacon-chain/state/stategen"
	ethpb "github.com/prysmaticlabs/prysm/proto/prysm/v1alpha1"
	"github.com/prysmaticlabs/prysm/proto/prysm/v1alpha1/wrapper"
	"github.com/prysmaticlabs/prysm/testing/require"
	"github.com/prysmaticlabs/prysm/testing/util"
)

func TestHeadSlot_DataRace(t *testing.T) {
	beaconDB := testDB.SetupDB(t)
	s := &Service{
		cfg: &config{BeaconDB: beaconDB},
	}
	b, err := wrapper.WrappedSignedBeaconBlock(util.NewBeaconBlock())
	require.NoError(t, err)
	st, _ := util.DeterministicGenesisState(t, 1)
	wait := make(chan struct{})
	go func() {
		defer close(wait)
		require.NoError(t, s.saveHead(context.Background(), [32]byte{}, b, st))
	}()
	s.HeadSlot()
	<-wait
}

func TestHeadRoot_DataRace(t *testing.T) {
	beaconDB := testDB.SetupDB(t)
	s := &Service{
		cfg:  &config{BeaconDB: beaconDB, StateGen: stategen.New(beaconDB)},
		head: &head{root: [32]byte{'A'}},
	}
	b, err := wrapper.WrappedSignedBeaconBlock(util.NewBeaconBlock())
	require.NoError(t, err)
	wait := make(chan struct{})
	st, _ := util.DeterministicGenesisState(t, 1)
	go func() {
		defer close(wait)
		require.NoError(t, s.saveHead(context.Background(), [32]byte{}, b, st))

	}()
	_, err = s.HeadRoot(context.Background())
	require.NoError(t, err)
	<-wait
}

func TestHeadBlock_DataRace(t *testing.T) {
	beaconDB := testDB.SetupDB(t)
	wsb, err := wrapper.WrappedSignedBeaconBlock(&ethpb.SignedBeaconBlock{})
	require.NoError(t, err)
	s := &Service{
		cfg:  &config{BeaconDB: beaconDB, StateGen: stategen.New(beaconDB)},
		head: &head{block: wsb},
	}
	b, err := wrapper.WrappedSignedBeaconBlock(util.NewBeaconBlock())
	require.NoError(t, err)
	wait := make(chan struct{})
	st, _ := util.DeterministicGenesisState(t, 1)
	go func() {
		defer close(wait)
		require.NoError(t, s.saveHead(context.Background(), [32]byte{}, b, st))

	}()
	_, err = s.HeadBlock(context.Background())
	require.NoError(t, err)
	<-wait
}

func TestHeadState_DataRace(t *testing.T) {
	beaconDB := testDB.SetupDB(t)
	s := &Service{
		cfg: &config{BeaconDB: beaconDB, StateGen: stategen.New(beaconDB)},
	}
	b, err := wrapper.WrappedSignedBeaconBlock(util.NewBeaconBlock())
	require.NoError(t, err)
	wait := make(chan struct{})
	st, _ := util.DeterministicGenesisState(t, 1)
	go func() {
		defer close(wait)
		require.NoError(t, s.saveHead(context.Background(), [32]byte{}, b, st))

	}()
	_, err = s.HeadState(context.Background())
	require.NoError(t, err)
	<-wait
}
