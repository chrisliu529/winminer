all:
	gcc mine.c -DLEVEL=$(level) -Wall -Werror -std=c11 -O3 -o mine-$(level)
