#include <stdio.h>
#include <stdlib.h>
#include <string.h>

int main(int argc, char **argv) {
    if (argc < 2) {
        printf("usage: ckit [--version|--help|exit N]\n");
        return 0;
    }
    if (strcmp(argv[1], "--version") == 0) {
        printf("ckit version 1.0.0\n");
    } else if (strcmp(argv[1], "--help") == 0) {
        printf("ckit — C example tool for shipsmoke\n");
    } else if (strcmp(argv[1], "exit") == 0 && argc > 2) {
        return atoi(argv[2]);
    } else {
        fprintf(stderr, "ckit: unknown command \"%s\"\n", argv[1]);
        return 2;
    }
    return 0;
}
