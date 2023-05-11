package ensign

// Metadata are user-defined key/value pairs that can be optionally added to an
// event to store/lookup data without unmarshaling the entire payload.
type Metadata map[string]string

// Get returns the metadata value for the given key. If the key is not in the metadata
// an empty string is returned without an error.
func (m Metadata) Get(key string) string {
	if val, ok := m[key]; ok {
		return val
	}
	return ""
}

// Set a metadata value for the given key; overwrites existing keys.
func (m Metadata) Set(key, value string) {
	m[key] = value
}
