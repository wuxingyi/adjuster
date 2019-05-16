#!/usr/bin/make -f

all: 
	go build adjuster.go

install: 
	install -D adjuster /usr/bin/adjuster

clean:
	rm -f adjuster

distclean: clean

uninstall:
	rm -f /usr/bin/adjuster

.PHONY: all clean distclean
