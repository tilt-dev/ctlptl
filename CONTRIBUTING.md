# Hacking on ctlptl

So you want to make a change to `ctlptl`!

## Contributing

We welcome contributions, either as bug reports, feature requests, or pull requests.

We want everyone to feel at home in this repo and its environs; please see our
[**Code of Conduct**](https://docs.tilt.dev/code_of_conduct.html) for some rules
that govern everyone's participation.

## Commands

Most of the commands for building and testing `ctlptl` should be familiar
with anyone used to developing in Golang. But we have a Makefile to wrap
common commands.

### Run

```
go run ./cmd/ctlptl
```

### Install dev version

```
make install
```

### Unit tests

```
make test
```

### Integration tests

```
make e2e
```

### Release

CircleCI will automatically build ctlptl releases when you push
a new tag to main.

```
git pull origin main
git fetch --tags
git tag -a v0.x.y -m "v0.x.y"
git push origin v0.x.y
```
