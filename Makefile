all:
	gcc mine.c -DLEVEL=$(level) -Wall -Werror -std=c11 -o mine-$(level)
