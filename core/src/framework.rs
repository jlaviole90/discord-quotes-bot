
pub struct Data {
    pub start_time: std::time::Instant,
    pub token: String,
}

pub type Error = Box<dyn std::error::Error + Send + Sync>;
pub type Context<'a> = poise::Context<'a, Data, Error>;
pub type CommandError = Error;
pub type FrameworkContext<'a> = poise::FrameworkContext<'a, Data, CommandError>;

pub async fn reply_syntax(ctx: Context<'_>, syntax: &str) {
    let _ = ctx.reply(syntax).await;
}
