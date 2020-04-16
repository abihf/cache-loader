# cache-loader
Golang Cache Loader

## Feature
 * Thread safe
 * Fetch once even when concurrent process request same key.
 * stale-while-revalidate when item is expired

## Example

```go
func main() {
  itemLoader := loader.NewLRU(fetchItem, 5 * time.Minute, 1000)
  item, err := loader.Get("key")
  // use item
}

func fetchItem(key interface{}) (interface{}, error) {
  res, err := http.Get("https://example.com/item/" + key)
  if err != nil {
    return nil, err
  }
  return processResponse(res)
}
```
