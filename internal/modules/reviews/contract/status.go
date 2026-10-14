package contract

func IsReleaseStatus(code string) bool {
	switch code {
	case "announced", "ongoing", "completed", "hiatus":
		return true
	default:
		return false
	}
}
