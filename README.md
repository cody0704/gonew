## gonew

本專案為 `golang.org/x/tools/cmd/gonew@v0.24.0` 的改良版本

將模板專案複製方式由 `go mod downloa` 改為直接使用 `git clone`

避免 Go module 處理 private repository 及 go mod cache 等相關問題

提供更直接且彈性的專案初始化流程

# Example

To install gonew:

```bash
go install github.com/cody0704/gonew@latest
```

To clone the basic command-line program template golang.org/x/example/hello
as your.domain/myprog, in the directory ./myprog:

```bash
gonew golang.org/x/example/hello your.domain/myprog
```

## Ref

- [https://github.com/golang/tools/blob/master/cmd/gonew/main.go](https://github.com/golang/tools/blob/master/cmd/gonew/main.go)
