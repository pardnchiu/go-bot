package line

type Option func(*options)

type options struct {
	path string
}

func WithPath(path string) Option {
	return func(o *options) {
		if path != "" {
			o.path = path
		}
	}
}
