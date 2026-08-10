use std::env;
use std::fs;
use std::io::{Read, Write};
use std::net::TcpListener;
use std::time::{SystemTime, UNIX_EPOCH};

fn main() {
    let arguments: Vec<String> = env::args().collect();
    if arguments.get(1).map(String::as_str) != Some("daemon")
        || arguments.get(2).map(String::as_str) != Some("start")
        || arguments.get(3).map(String::as_str) != Some("--foreground")
        || arguments.get(4).map(String::as_str) != Some("--internal-child")
    {
        eprintln!("expected daemon start --foreground --internal-child");
        std::process::exit(2);
    }
    let home = env::var("COMPOZY_HOME").expect("COMPOZY_HOME is required");
    let listener = TcpListener::bind(("127.0.0.1", 0)).expect("fake daemon binds");
    let port = listener.local_addr().expect("fake daemon address").port();
    let pid = std::process::id();
    let started_at = rfc3339_now();
    fs::create_dir_all(&home).expect("fake daemon home creates");
    fs::write(
        format!("{home}/daemon.json"),
        format!(
            "{{\"pid\":{pid},\"port\":{port},\"started_at\":\"{started_at}\"}}"
        ),
    )
    .expect("fake daemon record writes");
    let escaped_home = home.replace('\\', "\\\\").replace('"', "\\\"");
    let body = format!(
        "{{\"schema_version\":\"2026-07-16\",\"daemon\":{{\"pid\":{pid},\"started_at\":\"{started_at}\",\"http_host\":\"localhost\",\"http_port\":{port},\"user_home_dir\":\"{escaped_home}\",\"version\":\"0.3.0\"}}}}"
    );
    for connection in listener.incoming() {
        let mut connection = match connection {
            Ok(connection) => connection,
            Err(error) => {
                eprintln!("fake daemon accept: {error}");
                continue;
            }
        };
        let mut request = [0_u8; 2048];
        if let Err(error) = connection.read(&mut request) {
            eprintln!("fake daemon request: {error}");
            continue;
        }
        let response = format!(
            "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}",
            body.len()
        );
        if let Err(error) = connection.write_all(response.as_bytes()) {
            eprintln!("fake daemon response: {error}");
        }
    }
}

fn rfc3339_now() -> String {
    let seconds = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .expect("clock is after epoch")
        .as_secs();
    let days = (seconds / 86_400) as i64;
    let day_seconds = seconds % 86_400;
    let (year, month, day) = civil_from_days(days);
    let hour = day_seconds / 3_600;
    let minute = (day_seconds % 3_600) / 60;
    let second = day_seconds % 60;
    format!("{year:04}-{month:02}-{day:02}T{hour:02}:{minute:02}:{second:02}Z")
}

fn civil_from_days(days: i64) -> (i64, i64, i64) {
    let shifted = days + 719_468;
    let era = if shifted >= 0 { shifted } else { shifted - 146_096 } / 146_097;
    let day_of_era = shifted - era * 146_097;
    let year_of_era =
        (day_of_era - day_of_era / 1_460 + day_of_era / 36_524 - day_of_era / 146_096)
            / 365;
    let mut year = year_of_era + era * 400;
    let day_of_year = day_of_era - (365 * year_of_era + year_of_era / 4 - year_of_era / 100);
    let month_prime = (5 * day_of_year + 2) / 153;
    let day = day_of_year - (153 * month_prime + 2) / 5 + 1;
    let month = month_prime + if month_prime < 10 { 3 } else { -9 };
    year += i64::from(month <= 2);
    (year, month, day)
}
