package cachefile

import (
	"os"
	"slices"

	"github.com/sagernet/bbolt"
)

func (c *CacheFile) LoadCloudflareWARPRegistration(tag string) ([]byte, error) {
	var data []byte
	err := c.view(func(t *bbolt.Tx) error {
		bucket := c.bucket(t, bucketCloudflareWARP)
		if bucket == nil {
			return os.ErrNotExist
		}
		content := bucket.Get([]byte(tag))
		if len(content) == 0 {
			return os.ErrNotExist
		}
		data = slices.Clone(content)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (c *CacheFile) StoreCloudflareWARPRegistration(tag string, data []byte) error {
	return c.batch(func(t *bbolt.Tx) error {
		bucket, err := c.createBucket(t, bucketCloudflareWARP)
		if err != nil {
			return err
		}
		return bucket.Put([]byte(tag), data)
	})
}

func (c *CacheFile) DeleteCloudflareWARPRegistration(tag string) error {
	return c.batch(func(t *bbolt.Tx) error {
		bucket := c.bucket(t, bucketCloudflareWARP)
		if bucket == nil {
			return nil
		}
		return bucket.Delete([]byte(tag))
	})
}
