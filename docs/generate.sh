#!/bin/bash

asciidoctor-pdf order_entry/root.asciidoc -o OrderEntry.pdf
asciidoctor-pdf drop_copy/root.asciidoc -o DropCopy.pdf

asciidoctor -v order_entry/root.asciidoc -o OrderEntry.html
asciidoctor -v drop_copy/root.asciidoc -o DropCopy.html
