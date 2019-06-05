FROM geekidea/alpine-a:3.9
COPY eventer /
ENTRYPOINT ["/eventer"]
