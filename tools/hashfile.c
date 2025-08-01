// utility to compute SHA256 of a file and print it in hex.
// Compile with: gcc -o hashfile tools/hashfile.c -lcrypto
#include <stdio.h>
#include <stdlib.h>
#include <openssl/sha.h>

int main(int argc, char **argv) {
    if (argc != 2) {
        fprintf(stderr, "Usage: %s <file>\n", argv[0]);
        return 1;
    }
    const char *path = argv[1];
    FILE *f = fopen(path, "rb");
    if (!f) {
        perror("fopen");
        return 1;
    }
    unsigned char buf[8192];
    SHA256_CTX ctx;
    SHA256_Init(&ctx);
    size_t n;
    while ((n = fread(buf, 1, sizeof(buf), f)) > 0) {
        SHA256_Update(&ctx, buf, n);
    }
    fclose(f);
    unsigned char out[SHA256_DIGEST_LENGTH];
    SHA256_Final(out, &ctx);
    for (int i = 0; i < SHA256_DIGEST_LENGTH; i++) {
        printf("%02x", out[i]);
    }
    printf("  %s\n", path);
    return 0;
}
