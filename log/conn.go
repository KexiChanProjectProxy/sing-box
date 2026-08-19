package log

func OptionalString(key, value string) []Field {
	if value == "" {
		return nil
	}
	return []Field{String(key, value)}
}
