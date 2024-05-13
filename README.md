# Bitgarten tools

## Picture load

Tool to load a set of pictures into the database use load command:

```sh
picloadql -t 2 -T 2 -b 1GB <picture directory to load>
```

## Picture hashs

The tool generate a number of hashs for the image to identify double or similar pictures:

```sh
bin/darwin_arm64/imagehash -l 0
```

## Picture cleanup