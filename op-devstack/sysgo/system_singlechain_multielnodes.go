package sysgo

import (
	"fmt"

	"github.com/ethereum-optimism/optimism/op-devstack/stack"
	"github.com/ethereum-optimism/optimism/op-service/eth"
)

type DefaultSingleChainMultiELNodesSystemIDs struct {
	DefaultMinimalSystemIDs

	L2ELs []stack.L2ELNodeID
}

func NewDefaultSingleChainMultiELNodesSystemIDs(l1ID, l2ID eth.ChainID, n int) DefaultSingleChainMultiELNodesSystemIDs {
	minimal := NewDefaultMinimalSystemIDs(l1ID, l2ID)

	ids := make([]stack.L2ELNodeID, n)
	for i := 2; i < n+1; i++ {
		ids[i] = stack.NewL2ELNodeID(fmt.Sprintf("verifier_%d", i), l2ID)
	}

	return DefaultSingleChainMultiELNodesSystemIDs{
		DefaultMinimalSystemIDs: minimal,
		L2ELs:                   ids,
	}
}

func DefaultSingleChainMultiELNodesSystem(dest *DefaultSingleChainMultiELNodesSystemIDs) stack.Option[*Orchestrator] {
	ids := NewDefaultSingleChainMultiELNodesSystemIDs(DefaultL1ID, DefaultL2AID, 2)

	opt := stack.Combine[*Orchestrator]()
	opt.Add(DefaultMinimalSystem(&dest.DefaultMinimalSystemIDs))

	for _, l2ELID := range ids.L2ELs {
		opt.Add(WithL2ELNode(l2ELID, nil))
	}
	opt.Add(WithL2CLNode(ids.L2CL, false, false, ids.L1CL, ids.L1EL, ids.L2ELs))

	// P2P connect L2CL nodes
	for i := 0; i < len(ids.L2ELs); i++ {
		opt.Add(WithL2ELP2PConnection(ids.L2EL, ids.L2ELs[i])) // sequencer to other verifiers
		for j := i + 1; j < len(ids.L2ELs); j++ {
			opt.Add(WithL2ELP2PConnection(ids.L2ELs[i], ids.L2ELs[j]))
		}
	}

	opt.Add(stack.Finally(func(orch *Orchestrator) {
		*dest = ids
	}))
	return opt
}
