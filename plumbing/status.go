package plumbing

type StatusStage int

const (
	StatusCount StatusStage = iota
	StatusRead
	StatusFixChains
	StatusSort
	StatusDelta
	StatusSend
	StatusFetch
	StatusIndexHash
	StatusIndexCRC
	StatusIndexOffset
	StatusDone

	StatusUnknown StatusStage = -1
)

type StatusUpdate struct {
	Stage StatusStage

	ObjectsTotal int
	ObjectsDone  int
}

type StatusChan chan<- StatusUpdate

func (sc StatusChan) SendUpdate(update StatusUpdate) {
	if sc == nil {
		return
	}
	sc <- update
}

func (sc StatusChan) SendUpdateIfPossible(update StatusUpdate) {
	if sc == nil {
		return
	}
	if update.ObjectsDone == update.ObjectsTotal {
		sc <- update
		return
	}

	select {
	case sc <- update:
	default:
	}
}
