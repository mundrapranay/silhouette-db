fn main() {
    cbindgen::generate(std::env::var("CARGO_MANIFEST_DIR").unwrap())
        .expect("Unable to generate bindings")
        .write_to_file("frodopir_ffi.h");
}

