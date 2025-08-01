// Utility to inspect and verify a ShadowVault snapshot metadata JSON file.
// Validates the Ed25519 signature, summarizes contents, and checks local chunk availability.
// Build with:
//   cargo install --path .   # or compile standalone with `rustc` after adding dependencies manually
//
// Dependencies (add to Cargo.toml if using cargo):
// [dependencies]
// serde = { version = "1.0", features = ["derive"] }
// serde_json = "1.0"
// base64 = "0.21"
// ed25519-dalek = { version = "1.0", features = ["std"] }
// clap = { version = "4.2", features = ["derive"] }
// humantime = "2.1"

use std::fs::File;
use std::io::{BufReader, Read};
use std::path::{Path, PathBuf};
use std::collections::HashSet;

use clap::Parser;
use serde::Deserialize;
use base64::{engine::general_purpose, Engine as _};
use ed25519_dalek::{PublicKey, Signature, Verifier};
use std::fmt::Write as FmtWrite;
use std::fs;

#[derive(Parser, Debug)]
#[command(author, version, about = "Verify ShadowVault snapshot metadata and local chunk availability")]
struct Args {
    /// Path to snapshot metadata JSON (decrypted)
    #[arg(short, long)]
    snapshot: PathBuf,

    /// Base object storage directory where chunks live
    #[arg(short, long)]
    objects: PathBuf,

    /// Optionally override signer public key (base64) instead of using embedded signer_pub
    #[arg(long)]
    pubkey: Option<String>,

    /// Maximum number of missing chunk hashes to display
    #[arg(long, default_value_t = 20)]
    show_missing: usize,
}

#[derive(Deserialize)]
struct FileEntry {
    path: String,
    mode: u32,
    mod_time: String, // keep as string to preserve original encoding
    size: u64,
    chunk_hashes: Vec<String>,
}

#[derive(Deserialize)]
struct SnapshotMetadata {
    id: String,
    parent: Option<String>,
    timestamp: String,
    root: String,
    files: Vec<FileEntry>,
    signer_pub: String,
    signature: String,
}

fn canonical_snapshot_bytes(snap: &SnapshotMetadata) -> Vec<u8> {
    // Manually assemble JSON with deterministic ordering matching Go's json.Marshal of struct:
    // fields order: id, parent, timestamp, root, files, signer_pub
    let mut s = String::new();
    s.push('{');

    // "id"
    write!(s, "\"id\":{}", serde_json::to_string(&snap.id).unwrap()).unwrap();

    // "parent"
    if let Some(ref p) = snap.parent {
        write!(s, ",\"parent\":{}", serde_json::to_string(p).unwrap()).unwrap();
    } else {
        write!(s, ",\"parent\":null").unwrap();
    }

    // "timestamp"
    write!(s, ",\"timestamp\":{}", serde_json::to_string(&snap.timestamp).unwrap()).unwrap();

    // "root"
    write!(s, ",\"root\":{}", serde_json::to_string(&snap.root).unwrap()).unwrap();

    // "files"
    s.push_str(",\"files\":[");
    let mut first_file = true;
    for fe in &snap.files {
        if !first_file {
            s.push(',');
        }
        first_file = false;
        // file object: path, mode, mod_time, size, chunk_hashes
        s.push('{');
        write!(s, "\"path\":{}", serde_json::to_string(&fe.path).unwrap()).unwrap();
        write!(s, ",\"mode\":{}", fe.mode).unwrap();
        write!(s, ",\"mod_time\":{}", serde_json::to_string(&fe.mod_time).unwrap()).unwrap();
        write!(s, ",\"size\":{}", fe.size).unwrap();
        // chunk_hashes array
        s.push_str(",\"chunk_hashes\":[");
        let mut first_chunk = true;
        for ch in &fe.chunk_hashes {
            if !first_chunk {
                s.push(',');
            }
            first_chunk = false;
            write!(s, "{}", serde_json::to_string(ch).unwrap()).unwrap();
        }
        s.push(']');
        s.push('}');
    }
    s.push(']');

    // "signer_pub"
    write!(s, ",\"signer_pub\":{}", serde_json::to_string(&snap.signer_pub).unwrap()).unwrap();

    s.push('}');
    s.into_bytes()
}

fn chunk_exists(base: &Path, hash: &str) -> bool {
    // try direct
    let direct = base.join(hash);
    if direct.exists() {
        return true;
    }
    // try two-level split as <first2>/<rest>
    if hash.len() > 2 {
        let prefix = &hash[0..2];
        let rest = &hash[2..];
        let two = base.join(prefix).join(rest);
        if two.exists() {
            return true;
        }
    }
    false
}

fn main() -> anyhow::Result<()> {
    let args = Args::parse();

    let file = File::open(&args.snapshot)
        .map_err(|e| anyhow::anyhow!("failed to open snapshot file {}: {}", args.snapshot.display(), e))?;
    let mut reader = BufReader::new(file);
    let mut raw = String::new();
    reader.read_to_string(&mut raw)?;

    let snap: SnapshotMetadata = serde_json::from_str(&raw)
        .map_err(|e| anyhow::anyhow!("failed to parse snapshot JSON: {}", e))?;

    println!("Snapshot ID: {}", snap.id);
    if let Some(parent) = &snap.parent {
        println!("Parent: {}", parent);
    } else {
        println!("Parent: <none>");
    }
    println!("Root: {}", snap.root);
    println!("Timestamp: {}", snap.timestamp);
    println!("Files: {}", snap.files.len());

    let total_size: u64 = snap.files.iter().map(|f| f.size).sum();
    println!("Total declared byte size: {}", total_size);

    let signer_pub_b64 = args.pubkey.as_ref().map(|s| s.as_str()).unwrap_or(&snap.signer_pub);
    let signer_pub_bytes = general_purpose::STANDARD.decode(signer_pub_b64)
        .map_err(|e| anyhow::anyhow!("failed to decode signer_pub base64: {}", e))?;
    let public_key = PublicKey::from_bytes(&signer_pub_bytes)
        .map_err(|e| anyhow::anyhow!("invalid ed25519 public key: {}", e))?;

    let signature_bytes = general_purpose::STANDARD.decode(&snap.signature)
        .map_err(|e| anyhow::anyhow!("failed to decode signature base64: {}", e))?;
    let signature = Signature::from_bytes(&signature_bytes)
        .map_err(|e| anyhow::anyhow!("invalid signature format: {}", e))?;

    let canonical = canonical_snapshot_bytes(&snap);

    match public_key.verify(&canonical, &signature) {
        Ok(_) => println!("Signature: valid"),
        Err(e) => {
            println!("Signature: INVALID ({})", e);
            return Err(anyhow::anyhow!("signature verification failed"));
        }
    }

    // Collect all chunk hashes
    let mut all_chunks = HashSet::new();
    for fe in &snap.files {
        for ch in &fe.chunk_hashes {
            all_chunks.insert(ch.clone());
        }
    }
    println!("Unique chunks referenced: {}", all_chunks.len());

    // Check presence
    let mut missing = Vec::new();
    for ch in &all_chunks {
        if !chunk_exists(&args.objects, ch) {
            missing.push(ch.clone());
        }
    }

    if missing.is_empty() {
        println!("All chunks are present locally.");
    } else {
        println!("Missing chunks: {} (showing up to {})", missing.len(), args.show_missing);
        for ch in missing.iter().take(args.show_missing) {
            println!("  {}", ch);
        }
        if missing.len() > args.show_missing {
            println!("  ... and {} more", missing.len() - args.show_missing);
        }
    }

    Ok(())
}
