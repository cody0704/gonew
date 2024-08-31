## gonew

更改 gonew - golang.org/x/tools/cmd/gonew@latest 的方案

使用 git clone 的方式，避免 go module 對於 private repo 和 go mod cache 等等的問題

更直接相依於 git command

雖然 go mod cache 可以避免下載的問題
