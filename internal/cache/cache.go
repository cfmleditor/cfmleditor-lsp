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
	funcs    map[string]*scopeEntry // function name -> entry
	fileScope *scopeEntry           // file/component level
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

// PutFunc stores items for a function scope.
func (c *Cache) PutFunc(fileURI uri.URI, funcName string, hash uint64, items []protocol.CompletionItem) {
	c.mu.Lock()
	defer c.mu.Unlock()
	fc := c.getOrCreateFile(fileURI)
	fc.funcs[funcName] = &scopeEntry{Items: items, Hash: hash}
}

// GetFile returns cached items for the file/component scope, or nil on miss.
func (c *Cache) GetFile(fileURI uri.URI, hash uint64) []protocol.CompletionItem {
	c.mu.RLock()
	defer c.mu.RUnlock()
	fc := c.files[fileURI]
	if fc == nil || fc.fileScope == nil {
		return nil
	}
	if fc.fileScope.Hash == hash {
		return fc.fileScope.Items
	}
	return nil
}

// PutFile stores items for the file/component scope.
func (c *Cache) PutFile(fileURI uri.URI, hash uint64, items []protocol.CompletionItem) {
	c.mu.Lock()
	defer c.mu.Unlock()
	fc := c.getOrCreateFile(fileURI)
	fc.fileScope = &scopeEntry{Items: items, Hash: hash}
}

// Invalidate removes all cached entries for a file.
func (c *Cache) Invalidate(fileURI uri.URI) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.files, fileURI)
}

// HashScope computes a fast hash of content lines [startLine, endLine].
func HashScope(content string, startLine, endLine int) uint64 {
	lines := strings.SplitAfter(content, "\n")
	h := fnv.New64a()
	if startLine < 0 {
		startLine = 0
	}
	if endLine >= len(lines) {
		endLine = len(lines) - 1
	}
	for i := startLine; i <= endLine; i++ {
		h.Write([]byte(lines[i]))
	}
	return h.Sum64()
}
