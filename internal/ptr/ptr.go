package ptr

func PtrTo[T any](v T) *T {
	return &v
}

func StringFromPtr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func IntFromPtr(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

func BoolFromPtr(p *bool) bool {
	if p == nil {
		return false
	}
	return *p
}
