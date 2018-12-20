FROM geekidea/alpine-a:3.8
COPY eventer /

USER www:www
ENTRYPOINT ["/eventer"]
