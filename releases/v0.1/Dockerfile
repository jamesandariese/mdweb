FROM busybox

# Copy the local package files to the container's workspace.
ADD mdweb.linux-amd64 /mdweb
ADD site /site

ENTRYPOINT ["/mdweb"]
CMD ["-site", "/site"]

# Document that the service listens on port 4080.
EXPOSE 4080

