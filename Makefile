#!/usr/bin/make -f

all: adjuster

adjuster: adjuster.go
	go build adjuster.go

install: adjuster
	install -D adjuster /usr/bin/adjuster

clean:
	rm -f adjuster

distclean: clean

uninstall:
	rm -f /usr/bin/adjuster

.PHONY: all install clean distclean uninstall
