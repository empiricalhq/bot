package util

import (
	"container/list"
	"fmt"
	"regexp"
	"sync"
)

type RegexCache struct {
	mu    sync.Mutex
	cache map[string]*list.Element
	ll    *list.List
	size  int
}

type cacheEntry struct {
	pattern string
	regex   *regexp.Regexp
}

// NewRegexCache creates a new cache with a fixed size.
func NewRegexCache(size int) *RegexCache {
	if size <= 0 {
		size = 64
	}

	return &RegexCache{
		cache: make(map[string]*list.Element, size),
		ll:    list.New(),
		size:  size,
	}
}

func (c *RegexCache) Get(pattern string) (*regexp.Regexp, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.cache[pattern]; ok {
		c.ll.MoveToFront(elem)

		return elem.Value.(*cacheEntry).regex, nil
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to compile regex: %w", err)
	}

	if c.ll.Len() >= c.size {
		lruElem := c.ll.Back()
		if lruElem != nil {
			delete(c.cache, lruElem.Value.(*cacheEntry).pattern)
			c.ll.Remove(lruElem)
		}
	}

	elem := c.ll.PushFront(&cacheEntry{pattern: pattern, regex: re})
	c.cache[pattern] = elem

	return re, nil
}
