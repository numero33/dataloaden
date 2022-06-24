# dataloaden
![GitHub Workflow Status](https://img.shields.io/github/workflow/status/numero33/dataloaden/Test)
[![Go Report Card](https://goreportcard.com/badge/github.com/numero33/dataloaden)](https://goreportcard.com/report/github.com/numero33/dataloaden)
[![Coverage Status](https://coveralls.io/repos/github/numero33/dataloaden/badge.svg)](https://coveralls.io/github/numero33/dataloaden)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/numero33/dataloaden)
![GitHub top language](https://img.shields.io/github/languages/top/numero33/dataloaden)
![GitHub code size in bytes](https://img.shields.io/github/languages/code-size/numero33/dataloaden)
![GitHub](https://img.shields.io/github/license/numero33/dataloaden)

> This is an implementation of [Facebook's DataLoader](https://github.com/facebook/dataloader) in **Golang 1.18**.

Heavily inspired by [`vektah/dataloaden`](https://github.com/vektah/dataloaden)

## Install

```bash
go get -u github.com/numero33/dataloaden
```

## Usage

```golang

type User struct {
	ID   string
	Name string
}

// NewLoader will collect user requests for 2 milliseconds and send them as a single batch to the fetch func
// normally fetch would be a database call.
func NewLoader() *dataloaden.Loader[string, *User] {
	return dataloaden.NewLoader(dataloaden.LoaderConfig[string, *User]{
		Wait:     2 * time.Millisecond,
		MaxBatch: 100,
		Fetch: func(keys []string) ([]*User, []error) {
			users := make([]*User, len(keys))
			errors := make([]error, len(keys))

			for i, key := range keys {
				users[i] = &User{ID: key, Name: "user " + key}
			}
			return users, errors
		},
	})
}

func main() {
    userLoader := NewLoader()
    u, _ := dl.Load("E1")
}


```