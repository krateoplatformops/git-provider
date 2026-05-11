package utils

type unwrapper interface {
	Unwrap() error
}

func IsErr(target, err error) bool {
	for err != nil {
		if err == target {
			return true
		}
		u, ok := err.(unwrapper)
		if !ok {
			break
		}
		err = u.Unwrap()
	}
	return false
}
