Release dockerfiles go in here.

These build dockerfiles for the releases by downloading the static binary from
github and sticking them in a busybox container.

This saves quite a lot of room by not having the golang compiler in with the
released executable.

While this arguably doesn't actually matter in most cases, I prefer it because
it accurately captures the requirements of the project once released and I feel
that the rightness outweighs the laziness.
