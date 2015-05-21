FROM busybox

# Copy the local package files to the container's workspace.
ADD https://github.com/jamesandariese/mdweb/releases/download/0.1/mdweb.linux-amd64 /mdweb
ADD file://../../site /site

ENTRYPOINT ["/mdweb"]
CMD ["-site", "/site"]

# Document that the service listens on port 4080.
EXPOSE 4080

