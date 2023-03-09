# ziti-ci
Shared CI for Ziti projects

## GPG Keys
### Extending GPG Key expiration

1. Find the key

```
gpg --list-keys
```

2. Start key editor

```
gpg --edit-key <key-id-or-name>
```

3. List keys with `gpg> list` 
4. Select key with `gpg> key <key number`
5. Update expiration with `gpg> expire`
6. Save key with `gpg> save`

### Exporting a key
To export a key to be used by ziti-ci for commit signing...

1. Find the key you want to export:

```
gpg --list-keys
```

2. Export the keys

Replace $1 with the key id or name
```
gpg --armor --export $1
gpg --armor --export-secret-key $1
```

3. Take the output and put it in a GH secret
