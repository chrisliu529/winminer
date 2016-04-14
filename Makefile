all:
	gcc mine.c -DLEVEL=$(level) -Wall -Werror -o mine-$(level)
