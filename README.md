# Bitgarten tools

## List of tools


Column A | Column B
---------|----------
 picloadql | Load pictures and movies into database
 imagehash | create image hash to be able to compare images
 checkMedia | check media content (BLOB) if data is empty or if MD5 and SHA checksums are correct 
 hashclean | Check similar pictures and analyze HEIC content sub-pictures, if given then mark images to 'delete'  
 exifclean | evaluate image EXIF information and add corresponding EXIF data 
 exiftool | evaluate image EXIF information GPS information
 heic_thumb | HEIC thumbnail creation and scale images to thumbnail or bitgarten size 
 sync_album | synchronize album between two databases (source and destination) 
 tag_album |tag images referenced in Album with tag 'bitgarten' 
 videothumb | generate Video thumbnail 

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