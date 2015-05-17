all: photos.md

# If there's a photos-*.md file present, it should take the form
# ### Album Title
# somewhere inside of it.  Then if you make photos.md, this file
# will generate a new photos.md containing links to all the 
# different galleries available
photos.md: ALWAYS
	cp photos.md photos.md~
	(echo '## Photos'; for f in photos-*.md;do echo -n '['; X=`grep -F '### ' "$$f"|head -1`;echo "$${X#\### }]($${f%.md})";echo;done) > photos.md

.PHONY: ALWAYS
