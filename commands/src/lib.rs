use core::framework;
use poise::serenity_prelude as serenity;

pub mod quote;

#[poise::command(slash_command, prefix_command)]
pub async fn age(
    ctx: framework::Context<'_>,
    #[description = "Selected User"] user: Option<serenity::User>,
) -> Result<(), core::framework::Error> {
    let u = user.as_ref().unwrap_or_else(|| ctx.author());
    let resp = format!("{}'s account was created at {}", u.name, u.created_at());
    ctx.say(resp).await?;
    Ok(())
}

