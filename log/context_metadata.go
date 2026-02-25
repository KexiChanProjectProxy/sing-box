package log

import "context"

var defaultMetadataExtractor MetadataExtractor

// RegisterMetadataExtractor registers a metadata extractor function
// This should be called from adapter package to avoid import cycles
func RegisterMetadataExtractor(extractor MetadataExtractor) {
	defaultMetadataExtractor = extractor
}

// extractMetadata extracts metadata from context using the registered extractor
func extractMetadata(ctx context.Context) map[string]interface{} {
	if defaultMetadataExtractor != nil {
		return defaultMetadataExtractor(ctx)
	}
	return nil
}
