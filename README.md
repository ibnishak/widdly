widdly [![License](http://img.shields.io/:license-gpl3-blue.svg)](http://www.gnu.org/licenses/gpl-3.0.html) [![Build Status](https://travis-ci.org/opennota/widdly.png?branch=master)](https://travis-ci.org/opennota/widdly)
======

This is a minimal self-hosted app, written in Go, that can serve as a backend
for a personal [TiddlyWiki](http://tiddlywiki.com/).

## Requirements

Go 1.7+

## Installation

    go get -u github.com/opennota/widdly

## TODO

- [ ] `$:/DefaultTiddlers` loaded but not show up
- [x] add authorization back
- [ ] multiple TiddlyWiki in subpath/suburl
- [ ] ACL: login for read & edit, login for edit, all can edit


## Usage

Put `index.html` next to the executable (or, alternatively, embed `index.html`
into the executable by running `zip -9 - index.html | cat >> widdly`). Run:

    widdly -http :1337 -p letmein -db /path/to/the/database

- `-http :1337` - listen on port 1337 (by default port 8080 on localhost)
- `-p letmein` - protect by the password (optional); the username will be `widdly`.
- `-db /path/to/the/database` - explicitly specify which file to use for the
  database (by default `widdly.db` in the current directory)

## Build your own index.html

    git clone https://github.com/Jermolene/TiddlyWiki5
    cd TiddlyWiki5
    node tiddlywiki.js editions/empty --build index

Open `editions/empty/output/index.html` in a browser and install some plugins
(at the very least, the "TiddlyWeb and TiddlySpace components" plugin). You
will be prompted to save the updated index.html.

## TiddlyWiki base image

The TiddlyWiki code is stored in and served from index.html, which
(as you can see by clicking on the Tools tab) is TiddlyWiki version 5.1.17.

Plugins must be pre-baked into the TiddlyWiki file, not stored on the server
as lazily loaded Tiddlers. The index.html in this directory is 5.1.17 with
the TiddlyWeb and Markdown plugins added. The TiddlyWeb plugin is
required, so that index.html talks back to the server for content.

The process for preparing a new index.html is:

- Open tiddlywiki-5.1.17.html in your web browser.
- Click the control panel (gear) icon.
- Click the Plugins tab.
- Click "Get more plugins".
- Click "Open plugin library".
- Type "tiddlyweb" into the search box. The "TiddlyWeb and TiddlySpace components" should appear.
- Click Install. A bar at the top of the page should say "Please save and reload for the changes to take effect."
- edit `$:/plugins/tiddlywiki/tiddlyweb/save/offline` (need some time for loading & saving)
  - save openlist: `[all[]] -[[$:/HistoryList]] -[[$:/StoryList]] -[[$:/Import]] -[[$:/isEncrypted]] -[[$:/UploadName]] -[prefix[$:/state/]] -[prefix[$:/temp/]] -[field:bag[bag]] -[has[draft.of]]`
  - no openlist: `[all[]] -[[$:/HistoryList]] -[[$:/Import]] -[[$:/isEncrypted]] -[[$:/UploadName]] -[prefix[$:/state/]] -[prefix[$:/temp/]] -[field:bag[bag]] -[has[draft.of]]`
- Click the icon next to save, and an updated file will be downloaded.
- Open the downloaded file in the web browser.
- Repeat, adding any more plugins.
- Copy the final download to index.html.

## Similar projects

For a Google App Engine TiddlyWiki server, look at [rsc/tiddly](https://github.com/rsc/tiddly).
