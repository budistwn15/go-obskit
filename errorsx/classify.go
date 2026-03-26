package errorsx

const (
	LayerHandler     = "handler"
	LayerUsecase     = "usecase"
	LayerRepository  = "repository"
	LayerContract    = "contract"
	LayerIntegration = "integration"
	LayerJob         = "job"
)

func Classify(err error) Meta {
	if e, ok := Extract(err); ok {
		return e.Meta
	}
	return Meta{}
}
