widdly [![License](http://img.shields.io/:license-gpl3-blue.svg)](http://www.gnu.org/licenses/gpl-3.0.html) [![Build Status](https://travis-ci.org/opennota/widdly.png?branch=master)](https://travis-ci.org/opennota/widdly)
======

This is a minimal self-hosted app, written in Go, that can serve as a backend
for a personal [TiddlyWiki](http://tiddlywiki.com/).

## Requirements

Go 1.7+

## Build

get source

    $ git clone --depth=1 https://github.com/cs8425/widdly.git
    $ cd widdly

(optional) get dependency

    $ go get go.etcd.io/bbolt # bolt/bbolt support, cross-compile can work
    $ go get github.com/mattn/go-sqlite3 # sqlite support, won't work for cross-compile

build:

    $ go build .

or

    $ ./build_all.sh # build multi-arch executable binary to bin/widdly.*

## TODO

- [ ] `$:/DefaultTiddlers` loaded but not show up, might be cause by `$:/StoryList`
- [x] add authorization back
- [ ] multiple TiddlyWiki in subpath/suburl
- [ ] ACL: login for read & edit, login for edit, all can edit
- [x] check user/pass in file/db
- [ ] fix api_test.go & add more test
- [ ] set max keeping history revisions
  - [ ] flat file
  - [ ] bolt/bbolt
  - [ ] sqlite
- [x] send base html with gzip
- [x] select backend type without re-compile


## Usage

Setup account:

    ./widdly -u <username> -p <password> > user.lst
    ./widdly -u <username2> -p <password2> >> user.lst


Run:

    ./widdly -http :1337 -acc user.lst -db /path/to/the/database -gz 5

- `-http :1337` - listen on port 1337 (by default port 8080 on localhost)
- `-acc user.lst` - user list file.
- `-db /path/to/the/database` - explicitly specify which file to use for the database (by default `widdly.db` in the current directory)
- `-dbt flatFile` - database type: flatFile, bbolt, sqlite; use `-dbt ''` to list all
- `-gz 5` - gzip compress level (1~9), 0 for disable, -1 for golang default


## Different between PutSaver, TiddlyWeb and both enable

|                                      | PutSaver only [1]            | TiddlyWeb only                                                        | TiddlyWeb and PutSaver [2]  |
|--------------------------------------|------------------------------|-----------------------------------------------------------------------|-----------------------------|
| can install plugin                   | yes [3]                      | no, need update base file                                             | yes [4], click 'Save'       |
| update sending size                  | big, full html file (~2MB)   | little (~ tiddler's size)                                             | little, except click 'Save' |
| load tiddlers/configs from base file | once when page opened        | same as 'Save only'                                                   | same as 'Save only'         |
| load tiddlers/configs by ajax        | no                           | yes, can override base file values [5]                                | same as 'TiddlyWeb only'    |
| save tiddlers/configs into base file | yes [3][4]                   | no                                                                    | yes [4], click 'Save'       |
| save tiddlers/configs by ajax        | no                           | yes                                                                   | yes                         |
| loading timing                       | all in once when page opened | data in base file when page opened and then load others with ajax     | same as 'TiddlyWeb only'    |


- [1] base on WebDAV
- [2] this implement
- [3] need to disable all authorization in current implement (modify code), or use other WebDAV server
- [4] by using PutSaver (WebDAV), need login, cause a full upload of base file
- [5] `$:/StoryList` not work :(


## TiddlyWiki base image

The TiddlyWiki code is stored in and served from index.html, which
(as you can see by clicking on the Tools tab) is TiddlyWiki version 5.1.17.

Plugins must be pre-baked into the TiddlyWiki file, not stored on the server
as lazily loaded Tiddlers. The index.html in this directory is 5.1.17 with
the TiddlyWeb added. The TiddlyWeb plugin is required, so that index.html talks back to the server for content.

The process for preparing a new index.html is:

- Open tiddlywiki-5.1.17.html in your web browser.
- Click the control panel (gear) icon.
- Click the Plugins tab.
- Click "Get more plugins".
- Click "Open plugin library".
- Type "tiddlyweb" into the search box. The "TiddlyWeb and TiddlySpace components" should appear.
- Click Install. A bar at the top of the page should say "Please save and reload for the changes to take effect."
- edit `$:/plugins/tiddlywiki/tiddlyweb/save/offline` (need some time for loading & saving)
  - not save openlist: `[all[]] -[[$:/HistoryList]] -[[$:/StoryList]] -[[$:/Import]] -[[$:/isEncrypted]] -[[$:/UploadName]] -[prefix[$:/state/]] -[prefix[$:/temp/]] -[field:bag[bag]] -[has[draft.of]]`
  - save openlist: `[all[]] -[[$:/HistoryList]] -[[$:/Import]] -[[$:/isEncrypted]] -[[$:/UploadName]] -[prefix[$:/state/]] -[prefix[$:/temp/]] -[field:bag[bag]] -[has[draft.of]]`
- Click the icon next to save, and an updated file will be downloaded.
- Open the downloaded file in the web browser.
- Repeat, adding any more plugins. Or add it later when "widdly" start.
- Copy the final download to index.html.

## Similar projects

For a Google App Engine TiddlyWiki server, look at [rsc/tiddly](https://github.com/rsc/tiddly).
