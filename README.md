Assembling list of images in a dir recursively

Windows (CMD)

```
for /r %i in (*) do @echo %~fi >> file_list.txt
```

Bash

```
find . -type f -exec realpath {} \; > file_list.txt
```
# image-tagger
