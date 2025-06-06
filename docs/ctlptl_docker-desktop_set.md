## ctlptl docker-desktop set

Set the docker-desktop settings

### Synopsis

Set the docker-desktop settings

The first argument is the full path to the setting.

The second argument is the desired value.

Most settings are scalars. vm.fileSharing is a list of paths separated by commas.

```
ctlptl docker-desktop set KEY VALUE [flags]
```

### Examples

```
  ctlptl docker-desktop set vm.resources.cpus 2
   ctlptl docker-desktop set kubernetes.enabled false
  ctlptl docker-desktop set vm.fileSharing /Users,/Volumes,/private,/tmp
```

### Options

```
  -h, --help   help for set
```

### SEE ALSO

* [ctlptl docker-desktop](ctlptl_docker-desktop.md)	 - Debugging tool for the Docker Desktop client

###### Auto generated by spf13/cobra on 21-May-2025
