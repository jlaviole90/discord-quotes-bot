use poise::serenity_prelude as serenity;
use poise::serenity_prelude::Result;
use serenity::FullEvent as Event;

type Error = Box<dyn std::error::Error + Send + Sync>;

use core::framework;

pub async fn listen(_ctx: framework::FrameworkContext<'_>, event: &Event) -> Result<(), Error> {
    match event {
        _ => Ok(()),
    }
}

