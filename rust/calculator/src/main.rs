// 输入平均交易量和开始的天数，计算成本和分红
const BASE_RELEASE: f32 = 51200.0;
const BASE_DAMPING: f32 = 0.5;
const CHARGE: f32 = 2e-3;

fn period(day: u32) -> u32 {
    (day-1)/365+1
}

fn period_day_release(period: u32) -> f32 {
    BASE_RELEASE*(BASE_DAMPING.powf((period -1)as f32))
}

fn daily_release(day: u32) -> f32 {
    let period = period(day);
    period_day_release(period)
}

fn period_day(day: u32) -> u32 {
    let mut rest_day = day % 365;
    if rest_day == 0 {
        rest_day = 365;
    }
    rest_day
}

fn period_total_release(day: u32) -> f32 {
    let period_day = period_day(day);
    period_day as f32 * daily_release(day)
}


fn total_release(day: u32) -> f32 {
    let period: u32 = period(day);
    let mut total_release: f32 = 0.0;
    for p in 1..period+1 {
        match p {
            _period => total_release+=period_total_release(day),
            _ => total_release+= period_day_release(period)* 365.0,
        }
    }
    total_release
}


fn calculate_by_bonus(start_day_bonus: f32,start_day: u32, end_day: u32) -> ( f32, f32){
    let mut total_bonus: f32 = 0.0;
    let mut total_release: f32 = total_release(start_day);
    let start_day_release: f32 = daily_release(start_day);
    // let start_volume: f32 = start_day_bonus * start_day_release / CHARGE / 1e8;
    for d in start_day..end_day+1 {
        if d != start_day{
        total_release += daily_release(d);
        }
        total_bonus += start_day_bonus * start_day_release / total_release;
    }
    (total_release, total_bonus)
}

fn calculate_by_volume(avg_volum: f32,start_day: u32, end_day: u32) -> (f32, f32, f32) {
    let start_day_bonus: f32 = avg_volum*1e8*CHARGE/daily_release(start_day);
    let (total_release, total_bonus)=calculate_by_bonus(start_day_bonus, start_day,end_day);
    (total_release, total_bonus, total_bonus/(end_day-start_day+1) as f32)
}


fn main() {
    let start_day: u32 = 43;
    let end_day: u32 = 365;
    let avg_volume: f32 = 8.0;
    let (total_release, total_bonus, daily_bonus) = calculate_by_volume(avg_volume,start_day,end_day);
    println!("起始天数: {}\n结束天数: {}\n平均每天交易量: {}亿\n最后释放龙币数量: {}\n最后每个币总分红: ￥{}\n平均每天每币分红: ￥{}",start_day, end_day, avg_volume,total_release,total_bonus,  daily_bonus);
}

