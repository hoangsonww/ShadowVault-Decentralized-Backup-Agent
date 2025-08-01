// Utility to encrypt/decrypt a file using AES-256-GCM with a passphrase.
// Format (binary):
//   [4 bytes] magic "SVLT"
//   [1 byte ] version (0x01)
//   [16 bytes] salt
//   [12 bytes] nonce (IV)
//   [..] ciphertext (plaintext encrypted)
//   [16 bytes] GCM tag
//
// AAD is the prefix: magic + version + salt + nonce.
// Compile with:
//   g++ -std=c++17 -O2 -o aesgcm_file tools/aesgcm_file.cpp -lcrypto

#include <openssl/evp.h>
#include <openssl/rand.h>
#include <openssl/err.h>
#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <string>
#include <iostream>
#include <fstream>
#include <vector>

static constexpr size_t SALT_LEN = 16;
static constexpr size_t NONCE_LEN = 12;
static constexpr size_t TAG_LEN = 16;
static constexpr int PBKDF2_ITERS = 200000; // high enough for modest security
static constexpr size_t KEY_LEN = 32; // AES-256
static const unsigned char MAGIC[4] = {'S', 'V', 'L', 'T'};
static constexpr unsigned char VERSION = 0x1;

void print_openssl_errors() {
    ERR_print_errors_fp(stderr);
}

bool derive_key(const std::string &passphrase, const unsigned char *salt, unsigned char *out_key) {
    // PBKDF2-HMAC-SHA256
    if (!PKCS5_PBKDF2_HMAC(passphrase.c_str(), passphrase.size(),
                           salt, SALT_LEN,
                           PBKDF2_ITERS,
                           EVP_sha256(),
                           KEY_LEN,
                           out_key)) {
        print_openssl_errors();
        return false;
    }
    return true;
}

bool encrypt_file(const std::string &inpath, const std::string &outpath, const std::string &passphrase) {
    std::ifstream in(inpath, std::ios::binary);
    if (!in) {
        std::perror(("fopen " + inpath).c_str());
        return false;
    }
    std::ofstream out(outpath, std::ios::binary);
    if (!out) {
        std::perror(("fopen " + outpath).c_str());
        return false;
    }

    unsigned char salt[SALT_LEN];
    unsigned char nonce[NONCE_LEN];
    if (RAND_bytes(salt, SALT_LEN) != 1) {
        print_openssl_errors();
        return false;
    }
    if (RAND_bytes(nonce, NONCE_LEN) != 1) {
        print_openssl_errors();
        return false;
    }

    unsigned char key[KEY_LEN];
    if (!derive_key(passphrase, salt, key)) {
        fprintf(stderr, "key derivation failed\n");
        return false;
    }

    EVP_CIPHER_CTX *ctx = EVP_CIPHER_CTX_new();
    if (!ctx) {
        print_openssl_errors();
        return false;
    }

    if (1 != EVP_EncryptInit_ex(ctx, EVP_aes_256_gcm(), nullptr, nullptr, nullptr)) {
        print_openssl_errors();
        EVP_CIPHER_CTX_free(ctx);
        return false;
    }
    if (1 != EVP_CIPHER_CTX_ctrl(ctx, EVP_CTRL_GCM_SET_IVLEN, NONCE_LEN, nullptr)) {
        print_openssl_errors();
        EVP_CIPHER_CTX_free(ctx);
        return false;
    }
    if (1 != EVP_EncryptInit_ex(ctx, nullptr, nullptr, key, nonce)) {
        print_openssl_errors();
        EVP_CIPHER_CTX_free(ctx);
        return false;
    }

    // Prepare and write header: magic + version + salt + nonce
    unsigned char header[4 + 1 + SALT_LEN + NONCE_LEN];
    memcpy(header, MAGIC, 4);
    header[4] = VERSION;
    memcpy(header + 5, salt, SALT_LEN);
    memcpy(header + 5 + SALT_LEN, nonce, NONCE_LEN);
    // Use header as AAD
    int outlen;
    if (1 != EVP_EncryptUpdate(ctx, nullptr, &outlen, header, sizeof(header))) {
        print_openssl_errors();
        EVP_CIPHER_CTX_free(ctx);
        return false;
    }

    // Write header to output file
    out.write(reinterpret_cast<char*>(header), sizeof(header));

    // Encrypt file in chunks
    const size_t BUF_SIZE = 4096;
    std::vector<unsigned char> inbuf(BUF_SIZE);
    std::vector<unsigned char> outbuf(BUF_SIZE + EVP_CIPHER_block_size(EVP_aes_256_gcm()));
    while (in) {
        in.read(reinterpret_cast<char*>(inbuf.data()), BUF_SIZE);
        std::streamsize r = in.gcount();
        if (r > 0) {
            if (1 != EVP_EncryptUpdate(ctx, outbuf.data(), &outlen, inbuf.data(), r)) {
                print_openssl_errors();
                EVP_CIPHER_CTX_free(ctx);
                return false;
            }
            out.write(reinterpret_cast<char*>(outbuf.data()), outlen);
        }
    }

    // Finalize (for GCM this does not output additional plaintext)
    if (1 != EVP_EncryptFinal_ex(ctx, outbuf.data(), &outlen)) {
        print_openssl_errors();
        EVP_CIPHER_CTX_free(ctx);
        return false;
    }
    if (outlen > 0) {
        out.write(reinterpret_cast<char*>(outbuf.data()), outlen);
    }

    // Get tag
    unsigned char tag[TAG_LEN];
    if (1 != EVP_CIPHER_CTX_ctrl(ctx, EVP_CTRL_GCM_GET_TAG, TAG_LEN, tag)) {
        print_openssl_errors();
        EVP_CIPHER_CTX_free(ctx);
        return false;
    }

    // Append tag
    out.write(reinterpret_cast<char*>(tag), TAG_LEN);

    EVP_CIPHER_CTX_free(ctx);
    std::cout << "Encrypted " << inpath << " -> " << outpath << "\n";
    return true;
}

bool decrypt_file(const std::string &inpath, const std::string &outpath, const std::string &passphrase) {
    std::ifstream in(inpath, std::ios::binary);
    if (!in) {
        std::perror(("fopen " + inpath).c_str());
        return false;
    }
    std::ofstream out(outpath, std::ios::binary);
    if (!out) {
        std::perror(("fopen " + outpath).c_str());
        return false;
    }

    // Read header
    unsigned char header[4 + 1 + SALT_LEN + NONCE_LEN];
    if (!in.read(reinterpret_cast<char*>(header), sizeof(header))) {
        fprintf(stderr, "failed to read header\n");
        return false;
    }
    if (memcmp(header, MAGIC, 4) != 0) {
        fprintf(stderr, "magic mismatch\n");
        return false;
    }
    if (header[4] != VERSION) {
        fprintf(stderr, "unsupported version: %u\n", header[4]);
        return false;
    }
    unsigned char salt[SALT_LEN];
    unsigned char nonce[NONCE_LEN];
    memcpy(salt, header + 5, SALT_LEN);
    memcpy(nonce, header + 5 + SALT_LEN, NONCE_LEN);

    unsigned char key[KEY_LEN];
    if (!derive_key(passphrase, salt, key)) {
        fprintf(stderr, "key derivation failed\n");
        return false;
    }

    // Read rest of file into buffer (ciphertext + tag)
    std::vector<unsigned char> filebuf((std::istreambuf_iterator<char>(in)),
                                       std::istreambuf_iterator<char>());
    if (filebuf.size() < TAG_LEN) {
        fprintf(stderr, "file too short to contain tag\n");
        return false;
    }
    size_t ciphertext_len = filebuf.size() - TAG_LEN;
    unsigned char *ciphertext = filebuf.data();
    unsigned char *tag = filebuf.data() + ciphertext_len;

    EVP_CIPHER_CTX *ctx = EVP_CIPHER_CTX_new();
    if (!ctx) {
        print_openssl_errors();
        return false;
    }
    if (1 != EVP_DecryptInit_ex(ctx, EVP_aes_256_gcm(), nullptr, nullptr, nullptr)) {
        print_openssl_errors();
        EVP_CIPHER_CTX_free(ctx);
        return false;
    }
    if (1 != EVP_CIPHER_CTX_ctrl(ctx, EVP_CTRL_GCM_SET_IVLEN, NONCE_LEN, nullptr)) {
        print_openssl_errors();
        EVP_CIPHER_CTX_free(ctx);
        return false;
    }
    if (1 != EVP_DecryptInit_ex(ctx, nullptr, nullptr, key, nonce)) {
        print_openssl_errors();
        EVP_CIPHER_CTX_free(ctx);
        return false;
    }

    int outlen;
    // Set AAD (header)
    if (1 != EVP_DecryptUpdate(ctx, nullptr, &outlen, header, sizeof(header))) {
        print_openssl_errors();
        EVP_CIPHER_CTX_free(ctx);
        return false;
    }

    std::vector<unsigned char> outbuf(ciphertext_len + EVP_CIPHER_block_size(EVP_aes_256_gcm()));
    if (ciphertext_len > 0) {
        if (1 != EVP_DecryptUpdate(ctx, outbuf.data(), &outlen, ciphertext, ciphertext_len)) {
            print_openssl_errors();
            EVP_CIPHER_CTX_free(ctx);
            return false;
        }
        out.write(reinterpret_cast<char*>(outbuf.data()), outlen);
    }

    // Set expected tag before final
    if (1 != EVP_CIPHER_CTX_ctrl(ctx, EVP_CTRL_GCM_SET_TAG, TAG_LEN, tag)) {
        print_openssl_errors();
        EVP_CIPHER_CTX_free(ctx);
        return false;
    }

    // Finalize: returns 1 if tag verified, 0 otherwise
    int ret = EVP_DecryptFinal_ex(ctx, outbuf.data(), &outlen);
    EVP_CIPHER_CTX_free(ctx);
    if (ret <= 0) {
        fprintf(stderr, "decryption failed: authentication tag mismatch\n");
        return false;
    }
    if (outlen > 0) {
        out.write(reinterpret_cast<char*>(outbuf.data()), outlen);
    }

    std::cout << "Decrypted " << inpath << " -> " << outpath << "\n";
    return true;
}

void usage(const char *prog) {
    fprintf(stderr,
            "Usage:\n"
            "  %s -e|-d -p <passphrase> <infile> <outfile>\n"
            "    -e    encrypt\n"
            "    -d    decrypt\n"
            "    -p    passphrase\n", prog);
}

int main(int argc, char **argv) {
    if (argc != 5) {
        usage(argv[0]);
        return 1;
    }
    bool do_encrypt = false, do_decrypt = false;
    std::string pass;
    int argi = 1;
    for (; argi < argc; ++argi) {
        if (strcmp(argv[argi], "-e") == 0) {
            do_encrypt = true;
        } else if (strcmp(argv[argi], "-d") == 0) {
            do_decrypt = true;
        } else if (strcmp(argv[argi], "-p") == 0) {
            if (argi + 1 >= argc) {
                usage(argv[0]);
                return 1;
            }
            pass = argv[++argi];
        } else {
            break;
        }
    }
    if (do_encrypt == do_decrypt) {
        fprintf(stderr, "Specify exactly one of -e or -d\n");
        usage(argv[0]);
        return 1;
    }
    if (pass.empty()) {
        fprintf(stderr, "Passphrase required\n");
        usage(argv[0]);
        return 1;
    }
    if (argi + 2 != argc) {
        usage(argv[0]);
        return 1;
    }
    std::string infile = argv[argi];
    std::string outfile = argv[argi + 1];

    OpenSSL_add_all_algorithms();
    ERR_load_crypto_strings();

    bool ok = false;
    if (do_encrypt) {
        ok = encrypt_file(infile, outfile, pass);
    } else {
        ok = decrypt_file(infile, outfile, pass);
    }

    ERR_free_strings();
    EVP_cleanup();

    return ok ? 0 : 1;
}
