fn main() {
    let args: Vec<String> = std::env::args().skip(1).collect();
    if args.is_empty() {
        println!("usage: rustkit [--version|--help|exit N]");
        return;
    }
    match args[0].as_str() {
        "--version" => println!("rustkit version 1.0.0"),
        "--help" => println!("rustkit — Rust example tool for shipsmoke"),
        "exit" => {
            if let Some(n) = args.get(1) {
                std::process::exit(n.parse::<i32>().unwrap_or(1));
            }
        }
        other => {
            eprintln!("rustkit: unknown command {other:?}");
            std::process::exit(2);
        }
    }
}
