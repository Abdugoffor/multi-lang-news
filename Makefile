migrate:
	go run . migrate:create $(name)

# permissions-push:
# 	go run . permissions:push

# gen:
# 	go run ./tools/gen -name $(name) -table $(table) -fields "$(fields)"

# .PHONY: migrate permissions-push gen

run:
	go run main.go