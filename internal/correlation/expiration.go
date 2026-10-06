package correlation

import (
	"github.com/Coubiac/QueueAtlas/internal/model"
	"github.com/Coubiac/QueueAtlas/internal/parser/postfix"
)

// QueueExpiration reports an explicit qmgr observation for a candidate queue.
// It is not a recipient attempt, a global verdict or proof of log completeness.
type QueueExpiration struct {
	Ref          FactRef
	Timestamp    model.Timestamp
	NativeStatus string
}

func queueExpirationFrom(ref FactRef, o model.Observation) (QueueExpiration, bool) {
	if o.Kind != model.KindMessage || o.Service != "qmgr" || o.NoQueue || o.QueueID == "" || o.ParseError != "" {
		return QueueExpiration{}, false
	}
	status, present := o.Field("status")
	if !present || status != "expired" || !postfix.HasNativeStatus(o.Message, status) {
		return QueueExpiration{}, false
	}
	stamp := o.Timestamp
	if stamp.Value != nil {
		date := stamp.Value.UTC()
		stamp.Value = &date
	}
	return QueueExpiration{Ref: ref, Timestamp: stamp, NativeStatus: status}, true
}
