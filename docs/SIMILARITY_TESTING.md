# Testing Similarity Deduplication Effectiveness

This document explains how to verify that the `--similarity-deduplication` (`-sdd`) option is working effectively.

## How to Test the Feature

### 1. **Basic Usage with Logging**

```bash
# Standard crawler with similarity detection
./katana -u https://example.com -sdd -v

# With custom similarity threshold (more aggressive)
./katana -u https://example.com -sdd -st 0.05 -v

# Headless crawler with similarity detection and debug output  
./katana -u https://example.com -hl -sdd --debug
```

### 2. **Monitoring Output**

When similarity deduplication is active, you'll see:

- **Verbose messages** when content is filtered: 
  ```
  [similarity] Filtered similar content (score=0.156 > 0.100)
  ```

- **Debug messages** (`--debug`) show detailed analysis:
  ```
  [DBG] [similarity] Content analysis: 245 words, max_similarity=0.156, threshold=0.100
  [DBG] [similarity] Added unique document: this-is-a-unique-page
  [DBG] [similarity] Skipped https://example.com/page2 (similarity score: 0.863 > 0.100)
  ```

- **Final statistics** when crawling completes:
  ```
  [INF] Similarity detection: 103 processed, 32 unique, 71 filtered - 68.9% filter rate
  [INF] Crawl completed in 14s. 93 endpoints found.
  ```

### 3. **Effectiveness Indicators**

**Good filtering rate:** 10-40%
- Shows the feature is finding and filtering similar content
- Higher rates may indicate very similar site structure (e.g., e-commerce, pagination)

**Low filtering rate:** 0-5% 
- May indicate unique content across pages
- Could suggest threshold needs adjustment for your use case

**Very high filtering rate:** >60%
- May indicate threshold is too aggressive
- Could be missing unique content

### 4. **Comparing Results**

Run the same target with and without similarity deduplication:

```bash
# Without similarity deduplication
./katana -u https://example.com -o results_standard.txt

# With similarity deduplication  
./katana -u https://example.com -sdd -o results_similarity.txt

# To see detailed similarity analysis, use debug mode
./katana -u https://example.com -sdd --debug

# Compare results
echo "Standard: $(wc -l < results_standard.txt) URLs"
echo "Similarity: $(wc -l < results_similarity.txt) URLs" 
echo "Filtered: $(($(wc -l < results_standard.txt) - $(wc -l < results_similarity.txt))) URLs"
```

Use `--debug` to see exactly which URLs are being skipped:
```
[DBG] [similarity] Skipped https://example.com/similar-page (similarity score: 0.856 > 0.100)
```

### 5. **Test Sites for Verification**

**High similarity expected:**
- E-commerce sites (similar product pages)
- News sites (template-based articles)
- Documentation sites (similar page structures)
- Paginated content

**Low similarity expected:**
- Personal blogs (unique content)
- Corporate websites (varied page types)
- API documentation (different endpoints)

### 6. **Performance Testing**

```bash
# Measure time differences
time ./katana -u https://example.com -d 3 > /dev/null
time ./katana -u https://example.com -d 3 -sdd > /dev/null
```

Similarity deduplication should:
- **Reduce total crawl time** (fewer pages to process)
- **Use slightly more memory** (TF-IDF model storage)
- **Show processing overhead** (similarity calculations)

### 7. **Unit Tests**

Run the built-in tests to verify functionality:

```bash
go test ./pkg/utils/filters/ -v
```

Expected output shows filtering working:
```
=== RUN   TestSimilarityFilter
    similarity_test.go:56: Similarity filter stats: 4 processed, 3 unique, 1 filtered
--- PASS: TestSimilarityFilter (0.04s)
```

## Troubleshooting

### No Content Being Filtered

1. **Lower the threshold**: Try `-st 0.05` or `-st 0.02` for more aggressive filtering
2. **Try debug mode**: `--debug` to see similarity scores
3. **Verify content diversity**: Very unique content won't be filtered
4. **Check page size**: Very small pages may not have enough content for comparison

### Too Much Content Being Filtered

1. **Increase the threshold**: Try `-st 0.3` or `-st 0.5` for more permissive filtering
2. **Content might be genuinely similar** (template-based sites)
3. **Consider if this is actually desired behavior**
4. **Review debug output** to understand why pages are similar

### Memory Usage Concerns

**Built-in Safety Measures:**
- **Document Limit**: TF-IDF model is capped at 1,000 unique documents (FIFO removal)
- **Content Size Limit**: Documents over 10MB are skipped to prevent memory issues
- **Word Count Limit**: Documents over 50,000 words are truncated for processing
- **Hybrid Maps**: Use disk-backed storage that spills to disk when memory is full

**For Very Large Crawls:**
- Memory usage is bounded by the 1,000 document limit (~25-50MB typical)
- Monitor memory usage if crawling sites with extremely large pages
- The hybrid map storage helps manage memory automatically

## Configuration Options

### Similarity Threshold (`-st, --similarity-threshold`)

Control how strict the similarity detection is:

```bash
# Default threshold (0.1) - balanced filtering
./katana -u https://example.com -sdd

# Aggressive filtering (0.05) - filters more similar content  
./katana -u https://example.com -sdd -st 0.05

# Permissive filtering (0.3) - only filters very similar content
./katana -u https://example.com -sdd -st 0.3

# Very permissive (0.8) - only filters nearly identical content
./katana -u https://example.com -sdd -st 0.8
```

**Threshold Guidelines:**
- **0.01-0.05**: Very aggressive (filters content with slight similarities)
- **0.05-0.15**: Aggressive (good for template-heavy sites)
- **0.1**: Default balanced setting  
- **0.15-0.3**: Moderate (allows more content variations)
- **0.3-0.8**: Permissive (only filters very similar content)
- **0.8-1.0**: Very permissive (only filters near-identical content)

## Safety and Reliability

**Built-in Protection:**
- **Panic Recovery**: All TF-IDF calculations are wrapped with panic recovery
- **Division by Zero Prevention**: Safe handling of empty or zero-word documents  
- **Memory Bounds**: Document storage limited to prevent unbounded growth
- **Large Content Handling**: Very large pages (>10MB) are safely skipped
- **Thread Safety**: Full concurrent safety with proper mutex usage

**Production Ready:**
- No known memory leaks or crash conditions
- Graceful degradation on errors (falls back to accepting content)
- Resource cleanup on shutdown
- Safe for high-concurrency environments

## Expected Behavior

- **First page**: Always accepted (nothing to compare against)
- **Identical content**: Should be filtered (high similarity score)
- **Template-based pages**: May be filtered depending on content overlap
- **Unique content**: Should pass through unchanged
- **Empty/minimal content**: Always accepted (not enough for comparison)

## Output Examples

**Standard crawl (no similarity):**
```
[INF] Crawl completed in 20s. 88 endpoints found.
```

**With similarity deduplication:**
```
[INF] Similarity detection: 103 processed, 30 unique, 73 filtered - 70.9% filter rate
[INF] Crawl completed in 14s. 21 endpoints found.
```

Note how similarity deduplication:
- **Dramatically reduces endpoints found** (88 → 21) by filtering similar content
- **Reduces crawl time** (20s → 14s) by skipping similar pages  
- **Shows high filtering effectiveness** (73 out of 103 pages filtered, 70.9% filter rate)

The feature works best on sites with:
- Substantial text content per page
- Some level of content similarity (templates, repeated sections)
- Multiple pages with similar structure but different specific information
