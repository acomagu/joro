# joro: Make JSON Unix friendly

joro transforms JSON into discrete assignments to make it easier to handle with Unix commands, `grep`, `column`, `awk` and so on.

```
$ curl "https://api.github.com/repos/tomnomnom/gron/commits?per_page=1" | joro
#0 author avatar_url https://avatars1.githubusercontent.com/u/58276?v=4
#0 author following_url https://api.github.com/users/tomnomnom/following{/other_user}
#0 author gists_url https://api.github.com/users/tomnomnom/gists{/gist_id}
#0 author login tomnomnom
#0 author id <58276>
#0 author gravatar_id <>
#0 author followers_url https://api.github.com/users/tomnomnom/followers
#0 author repos_url https://api.github.com/users/tomnomnom/repos
#0 author url https://api.github.com/users/tomnomnom
#0 author starred_url https://api.github.com/users/tomnomnom/starred{/owner}{/repo}
...
```

To work backwords, use `joro -v`.

```
$ curl "https://api.github.com/repos/tomnomnom/gron/commits?per_page=1" | joro --no-padding | grep 'commit author' | joro -v | jq
[
  {
    "commit": {
      "author": {
        "date": "2018-05-01T12:13:11Z",
        "email": "mail@tomnomnom.com",
        "name": "Tom Hudson"
      }
    }
  }
]
```

Help:

```
$ joro --help
Usage:
  -from-joro
        parse input as Joro rows instead of JSON
  -from-json
        parse input as JSON (default)
  -j    parse input as Joro rows instead of JSON (shorthand)
  -no-padding
        without space padding
  -p    without space padding (shorthand)
  -padding
        with space padding (default)
  -t    convert to table format (shorthand)
  -table
        convert to table format
  -v    convert from Joro rows to JSON
```
