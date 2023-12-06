# Website Performance Tools

This is a kitchen-sink repository for my personal website performance tools.

## Find Unused CSS

This tool crawls a website to find unused CSS classes based on a provided stylesheet.

### Install

```bash
go install github.com/bounoable/siteperf/cmd/find-unused-css
```

### Example

To check for unused CSS classes, run this command:

```bash
find-unused-css -url google.com -css style.css -limit 100 -out unusued.txt
```

This command checks the first 100 pages of google.com for CSS classes in
style.css that aren't used and saves them to unused.txt. Each line in
unused.txt lists an unused class name.

## License

[MIT](./LICENSE)
