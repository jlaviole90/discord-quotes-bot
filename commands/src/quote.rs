use core::framework;
use poise::serenity_prelude::utils;

#[poise::command(slash_command, prefix_command)]
pub async fn quote(
    ctx: framework::Context<'_>,
) -> Result<(), framework::Error> {
    // invocation_string() is just the command text
    
    ctx.say(ctx.).await?;
    
    //let channels = ctx.guild().unwrap().channels.clone();
    //channels.into_iter().find(|c| c.name.eq_ignore_ascii_case("quotes")).unwrap().send_message(CreateMessage::);

    return Ok(());
}

