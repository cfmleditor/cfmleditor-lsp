// Package cache provides per-file scoped completion caching.
package cache

import (
	"hash/fnv"
	"strings"
	"sync"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// FuncRange represents a function's line boundaries in a file.
type FuncRange struct {
	Name  string
	Start int
	End   int
}

// scopeEntry holds cached items for one scope within a file.
type scopeEntry struct {
	Items []protocol.CompletionItem
	Hash  uint64
}

// FileCache holds all scoped completion entries for a single file.
type FileCache struct {
	funcs     map[string]*scopeEntry // function name -> entry
	fileScope *scopeEntry            // file/component level
}

// Cache stores per-file completion caches.
type Cache struct {
	mu    sync.RWMutex
	files map[uri.URI]*FileCache
}

// New creates an empty Cache.
func New() *Cache {
	return &Cache{files: make(map[uri.URI]*FileCache)}
}

func (c *Cache) getOrCreateFile(fileURI uri.URI) *FileCache {
	fc := c.files[fileURI]
	if fc == nil {
		fc = &FileCache{funcs: make(map[string]*scopeEntry)}
		c.files[fileURI] = fc
	}

	return fc
}

// GetFunc returns cached items for a function scope, or nil on miss.
func (c *Cache) GetFunc(fileURI uri.URI, funcName string, hash uint64) []protocol.CompletionItem {
	c.mu.RLock()
	defer c.mu.RUnlock()

	fc := c.files[fileURI]
	if fc == nil {
		return nil
	}

	e := fc.funcs[funcName]
	if e != nil && e.Hash == hash {
		return e.Items
	}

	return nil
}

// GetFuncStale returns cached items for a function scope ignoring hash (stale OK).
func (c *Cache) GetFuncStale(fileURI uri.URI, funcName string) []protocol.CompletionItem {
	c.mu.RLock()
	defer c.mu.RUnlock()

	fc := c.files[fileURI]
	if fc == nil {
		return nil
	}

	e := fc.funcs[funcName]
	if e != nil {
		return e.Items
	}

	return nil
}

// PutFunc stores items for a function scope.
func (c *Cache) PutFunc(fileURI uri.URI, funcName string, hash uint64, items []protocol.CompletionItem) {
	c.mu.Lock()
	defer c.mu.Unlock()

	fc := c.getOrCreateFile(fileURI)
	fc.funcs[funcName] = &scopeEntry{Items: items, Hash: hash}
}

// GetFile returns cached items for the file/component scope, or nil if not set.
func (c *Cache) GetFile(fileURI uri.URI) []protocol.CompletionItem {
	c.mu.RLock()
	defer c.mu.RUnlock()

	fc := c.files[fileURI]
	if fc == nil || fc.fileScope == nil {
		return nil
	}

	return fc.fileScope.Items
}

// PutFile stores items for the file/component scope.
func (c *Cache) PutFile(fileURI uri.URI, items []protocol.CompletionItem) {
	c.mu.Lock()
	defer c.mu.Unlock()

	fc := c.getOrCreateFile(fileURI)
	fc.fileScope = &scopeEntry{Items: items}
}

// Invalidate removes all cached entries for a file.
func (c *Cache) Invalidate(fileURI uri.URI) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.files, fileURI)
}

// InvalidateAll removes all cached entries.
func (c *Cache) InvalidateAll() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.files = make(map[uri.URI]*FileCache)
}

// HashScope computes a fast hash of content lines [startLine, endLine].
func HashScope(content string, startLine, endLine int) uint64 {
	h := fnv.New64a()
	line := 0
	i := 0
	// Skip to startLine
	for line < startLine && i < len(content) {
		idx := strings.IndexByte(content[i:], '\n')
		if idx < 0 {
			return h.Sum64()
		}

		i += idx + 1
		line++
	}
	// Hash lines [startLine, endLine]
	for line <= endLine && i < len(content) {
		idx := strings.IndexByte(content[i:], '\n')
		if idx < 0 {
			h.Write([]byte(content[i:]))

			break
		}

		h.Write([]byte(content[i : i+idx+1]))
		i += idx + 1
		line++
	}

	return h.Sum64()
}
