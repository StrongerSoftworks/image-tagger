# image-tagger

## Helpful Commands

Creating link to local image dir:

Windows (CMD) elevated permissions

```
mklink /d D:\dev\github.com\stronger-softworks\image-tagger\images  D:\dev\github.com\stronger-softworks\picture-website\public\images
```

Assembling list of images in a dir recursively:

Windows (CMD)

```
for /r %i in (*) do @echo %~fi >> file_list.txt
```

Bash

```
find . -type f -exec realpath {} \; > file_list.txt
```
