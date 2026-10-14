package reviews

// Status is a fixed reference value. These lists are not stored and cannot be extended through the API.
type Status struct {
	Code  string
	Label string
}

var publicationStatuses = []Status{
	{Code: "draft", Label: "Черновик"},
	{Code: "published", Label: "Опубликован"},
}

var releaseStatuses = []Status{
	{Code: "announced", Label: "Анонс"},
	{Code: "ongoing", Label: "Онгоинг"},
	{Code: "completed", Label: "Завершено"},
	{Code: "hiatus", Label: "Перерыв"},
}

func PublicationStatuses() []Status {
	return cloneStatuses(publicationStatuses)
}

func ReleaseStatuses() []Status {
	return cloneStatuses(releaseStatuses)
}

func cloneStatuses(src []Status) []Status {
	out := make([]Status, len(src))
	copy(out, src)
	return out
}
