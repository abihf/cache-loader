# cache-loader
![GitHub tag (latest SemVer)](https://img.shields.io/github/v/tag/abihf/cache-loader?label=version)
[![go.dev reference](https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white)](https://pkg.go.dev/github.com/abihf/cache-loader)
![Go](https://github.com/abihf/cache-loader/workflows/Go/badge.svg)
[![codecov](https://codecov.io/gh/abihf/cache-loader/branch/master/graph/badge.svg)](https://codecov.io/gh/abihf/cache-loader)

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
