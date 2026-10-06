package correlation

import (
	"crypto/sha256"
	"encoding/hex"
)

// ContinuityInstances binds candidate keys to an explicit claim context. Plan is
// an internally checked assertion, not a physical proof. Partition preserves all
// independent generations and their uncertainty; no boundary joins queues.
// Keep the plan with the partition when interpreting these contextual keys.
type ContinuityInstances struct {
	Partition InstancePartition
	Plan      ContinuityPlan
}

// BuildQueueInstancesWithContinuity checks claims against the complete supplied
// snapshot before binding all candidate keys to facts AND claims. It never
// accepts a caller-built ContinuityPlan as a validation token. Even an empty
// explicit claim context has a distinct revision from BuildQueueInstances.
// On error there is no partial plan, partition or revision. Facts must remain
// immutable during the call, as must claims. Output slices and pointers are owned
// by the result.
func BuildQueueInstancesWithContinuity(facts []Fact, limit int, claims ContinuityClaims) (ContinuityInstances, error) {
	plan, err := CheckContinuityClaims(facts, limit, claims)
	if err != nil {
		return ContinuityInstances{}, err
	}
	partition, err := BuildQueueInstances(facts, limit)
	if err != nil {
		return ContinuityInstances{}, err
	}
	h := sha256.New()
	frame := revisionFramer{h}
	frame.text("queue-instances-with-continuity-v1")
	frame.text(plan.Revision)
	partition.Revision = hex.EncodeToString(h.Sum(nil))
	for i := range partition.Instances {
		partition.Instances[i].Key.Revision = partition.Revision
	}
	return ContinuityInstances{Partition: partition, Plan: plan}, nil
}
